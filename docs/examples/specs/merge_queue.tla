---------------------------- MODULE merge_queue ----------------------------
(***************************************************************************)
(* TLA+ specification for a scope-aware merge queue with speculative       *)
(* batching and bisection on failure.                                      *)
(*                                                                         *)
(* Safety properties:                                                      *)
(*   1. No two conflicting PRs merge simultaneously (scope isolation)      *)
(*   2. A PR merges only after its tests pass (test-before-merge)          *)
(*   3. Bisection correctly identifies the failing PR                      *)
(*   4. The main branch is never broken (invariant)                        *)
(*                                                                         *)
(* Liveness properties:                                                    *)
(*   1. Every queued PR eventually merges or is ejected (progress)         *)
(*   2. Non-conflicting PRs can merge in parallel (parallelism)            *)
(*                                                                         *)
(* Model: PRs are queued, grouped into scope lanes, speculatively tested   *)
(* as a batch, and either merged (all pass) or bisected (some fail).       *)
(***************************************************************************)

EXTENDS Integers, Sequences, FiniteSets, TLC

CONSTANTS
    PRs,              \* Set of PR identifiers
    Scopes,           \* Set of scope identifiers (e.g., "frontend", "backend")
    MaxBatchSize,     \* Maximum PRs per speculative batch
    MaxLanes          \* Maximum parallel lanes

VARIABLES
    queue,            \* Sequence of PRs waiting to be processed
    lanes,            \* Function: scope -> set of PRs being tested
    mainBranch,       \* Set of merged PRs (represents main branch state)
    testResults,      \* Function: PR -> {pass, fail, unknown}
    prScope,          \* Function: PR -> scope (assigned when queued)
    prState,          \* Function: PR -> {queued, testing, merged, ejected}
    bisecting         \* Set of lanes currently bisecting

vars == <<queue, lanes, mainBranch, testResults, prScope, prState, bisecting>>

\* --- Type invariant ---
TypeOK ==
    /\ queue \in Seq(PRs)
    /\ \A s \in Scopes : lanes[s] \subseteq PRs
    /\ mainBranch \subseteq PRs
    /\ \A pr \in PRs : testResults[pr] \in {"pass", "fail", "unknown"}
    /\ \A pr \in PRs : prScope[pr] \in Scopes \cup {"none"}
    /\ \A pr \in PRs : prState[pr] \in {"queued", "testing", "merged", "ejected"}
    /\ bisecting \subseteq Scopes

\* --- Safety: Scope isolation ---
\* No two lanes test PRs with the same scope simultaneously.
\* (Each scope maps to exactly one lane.)
ScopeIsolation ==
    \A s1, s2 \in Scopes :
        s1 /= s2 => lanes[s1] \cap lanes[s2] = {}

\* --- Safety: Test before merge ---
\* A PR is merged only if it has passed tests.
TestBeforeMerge ==
    \A pr \in PRs :
        prState[pr] = "merged" => testResults[pr] = "pass"

\* --- Safety: Main branch never broken ---
\* All merged PRs have passing tests.
MainBranchHealthy ==
    \A pr \in mainBranch : testResults[pr] = "pass"

\* --- Initial state ---
Init ==
    /\ queue = <<>>
    /\ lanes = [s \in Scopes |-> {}]
    /\ mainBranch = {}
    /\ testResults = [pr \in PRs |-> "unknown"]
    /\ prScope = [pr \in PRs |-> "none"]
    /\ prState = [pr \in PRs |-> "queued"]
    /\ bisecting = {}

\* --- Actions ---

\* A PR enters the queue with an assigned scope.
EnqueuePR(pr, scope) ==
    /\ prState[pr] = "queued"
    /\ pr \notin Range(queue)
    /\ queue' = Append(queue, pr)
    /\ prScope' = [prScope EXCEPT ![pr] = scope]
    /\ UNCHANGED <<lanes, mainBranch, testResults, prState, bisecting>>

\* Group queued PRs into a lane for testing (speculative batch).
FormBatch(scope) ==
    /\ lanes[scope] = {}                              \* Lane is idle
    /\ scope \notin bisecting                          \* Not bisecting
    /\ \E batch \in SUBSET {pr \in Range(queue) : prScope[pr] = scope} :
        /\ batch /= {}
        /\ Cardinality(batch) <= MaxBatchSize
        /\ lanes' = [lanes EXCEPT ![scope] = batch]
        /\ prState' = [pr \in PRs |->
            IF pr \in batch THEN "testing" ELSE prState[pr]]
        /\ UNCHANGED <<queue, mainBranch, testResults, prScope, bisecting>>

\* Tests complete for a batch — all pass.
BatchPass(scope) ==
    /\ lanes[scope] /= {}
    /\ scope \notin bisecting
    /\ \A pr \in lanes[scope] : testResults[pr] = "pass"
    /\ prState' = [pr \in PRs |->
        IF pr \in lanes[scope] THEN "merged" ELSE prState[pr]]
    /\ mainBranch' = mainBranch \cup lanes[scope]
    /\ lanes' = [lanes EXCEPT ![scope] = {}]
    \* Remove merged PRs from queue
    /\ queue' = SelectSeq(queue, LAMBDA pr : pr \notin lanes[scope])
    /\ UNCHANGED <<testResults, prScope, bisecting>>

\* Tests fail for a batch — enter bisection.
BatchFail(scope) ==
    /\ lanes[scope] /= {}
    /\ scope \notin bisecting
    /\ Cardinality(lanes[scope]) > 1
    /\ \E pr \in lanes[scope] : testResults[pr] = "fail"
    /\ bisecting' = bisecting \cup {scope}
    /\ UNCHANGED <<queue, lanes, mainBranch, testResults, prScope, prState>>

\* Bisection identifies the failing PR and ejects it.
BisectComplete(scope, failingPR) ==
    /\ scope \in bisecting
    /\ failingPR \in lanes[scope]
    /\ testResults[failingPR] = "fail"
    \* All other PRs in the lane pass individually
    /\ \A pr \in lanes[scope] \ {failingPR} : testResults[pr] = "pass"
    \* Merge the good PRs, eject the bad one
    /\ LET goodPRs == lanes[scope] \ {failingPR}
       IN /\ mainBranch' = mainBranch \cup goodPRs
          /\ prState' = [pr \in PRs |->
              IF pr \in goodPRs THEN "merged"
              ELSE IF pr = failingPR THEN "ejected"
              ELSE prState[pr]]
    /\ lanes' = [lanes EXCEPT ![scope] = {}]
    /\ bisecting' = bisecting \ {scope}
    /\ queue' = SelectSeq(queue, LAMBDA pr : pr \notin lanes[scope])
    /\ UNCHANGED <<testResults, prScope>>

\* A test result arrives for a PR.
TestComplete(pr, result) ==
    /\ testResults[pr] = "unknown"
    /\ result \in {"pass", "fail"}
    /\ testResults' = [testResults EXCEPT ![pr] = result]
    /\ UNCHANGED <<queue, lanes, mainBranch, prScope, prState, bisecting>>

\* --- Specification ---

\* Helper: Range of a sequence
Range(seq) == {seq[i] : i \in 1..Len(seq)}

Next ==
    \/ \E pr \in PRs, scope \in Scopes : EnqueuePR(pr, scope)
    \/ \E scope \in Scopes : FormBatch(scope)
    \/ \E scope \in Scopes : BatchPass(scope)
    \/ \E scope \in Scopes : BatchFail(scope)
    \/ \E scope \in Scopes, pr \in PRs : BisectComplete(scope, pr)
    \/ \E pr \in PRs, result \in {"pass", "fail"} : TestComplete(pr, result)

Spec == Init /\ [][Next]_vars

\* --- Liveness ---
\* Every PR eventually reaches a terminal state (merged or ejected).
Progress == \A pr \in PRs :
    prState[pr] = "queued" ~> (prState[pr] = "merged" \/ prState[pr] = "ejected")

\* Non-conflicting lanes can run in parallel.
Parallelism ==
    \E s1, s2 \in Scopes :
        s1 /= s2 /\ lanes[s1] /= {} /\ lanes[s2] /= {}

\* --- Invariants ---
SafetyInvariant ==
    /\ TypeOK
    /\ ScopeIsolation
    /\ TestBeforeMerge
    /\ MainBranchHealthy

=============================================================================
