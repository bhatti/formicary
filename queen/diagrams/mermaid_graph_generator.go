package diagrams

import (
	"fmt"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
	"strings"
)

const (
	endSuccessColor = "success"
	endFailColor    = "error"
	blueColor       = "primary"
	defaultColor    = "default"
	processingColor = "warning"
)

// MermaidGenerator structure for Mermaid diagrams
type MermaidGenerator struct {
	jobDefinition   *types.JobDefinition
	jobExecution    *types.JobExecution
	executionStatus common.RequestState
	dupTasks        map[string]bool
	buf             strings.Builder
	nodeStyles      map[string]string
	edgeCounter     int
}

// NewMermaid constructor
func NewMermaid(
	jobDefinition *types.JobDefinition,
	jobExecution *types.JobExecution) (*MermaidGenerator, error) {
	if jobDefinition == nil {
		return nil, fmt.Errorf("job jobDefinition is not specified")
	}
	if err := jobDefinition.Validate(); err != nil {
		return nil, err
	}
	executionStatus := common.PENDING
	if jobExecution != nil {
		executionStatus = jobExecution.JobState
	}
	return &MermaidGenerator{
		jobDefinition:   jobDefinition,
		jobExecution:    jobExecution,
		executionStatus: executionStatus,
		dupTasks:        make(map[string]bool),
		nodeStyles:      make(map[string]string),
		edgeCounter:     0,
	}, nil
}

// GenerateMermaid creates mermaid flowchart config
func (mg *MermaidGenerator) GenerateMermaid() (string, error) {
	endColor := blueColor
	if mg.executionStatus.Completed() {
		endColor = endSuccessColor
	} else if mg.executionStatus.Failed() {
		endColor = endFailColor
	}

	head, nodes, err := mg.buildTree()
	if err != nil {
		return "", err
	}

	// Start mermaid flowchart
	mg.write("flowchart TD\n")

	// Define nodes
	if mg.jobExecution != nil {
		mg.write("    START{\"START\"}\n")
		if mg.jobExecution.JobState.IsTerminal() {
			mg.write("    END[\"END\"]\n")
		}
		mg.addNodeStyle("START", blueColor, true)
		if mg.jobExecution.JobState.IsTerminal() {
			mg.addNodeStyle("END", endColor, true)
		}
	}

	// Write all task nodes
	for _, node := range nodes {
		mg.writeNode(node)
	}

	// Write connections
	if mg.jobExecution != nil {
		mg.write("    START --> " + mg.sanitizeNodeId(head.task.TaskType) + "\n")
	}

	// Write task connections recursively
	mg.writeTask(head)

	// Connect to end node
	if mg.jobExecution != nil && mg.jobExecution.JobState.IsTerminal() {
		lastType := "last"
		lastExecTask := mg.jobExecution.GetLastExecutedTask()
		if lastExecTask == nil {
			lastTask := mg.jobDefinition.GetLastTask()
			if lastTask != nil {
				lastType = lastTask.TaskType
			}
		} else {
			lastType = lastExecTask.TaskType
		}

		if endColor == blueColor {
			mg.write(fmt.Sprintf("    %s -.-> END\n", mg.sanitizeNodeId(lastType)))
		} else {
			mg.write(fmt.Sprintf("    %s --> END\n", mg.sanitizeNodeId(lastType)))
		}
	}

	// Add CSS classes for styling
	mg.writeStyles()

	return mg.buf.String(), nil
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////

func (mg *MermaidGenerator) writeTask(node *Node) {
	if node == nil {
		return
	}
	for _, child := range node.children {
		key := toKey(node.task.TaskType, child.task.TaskType)
		if mg.dupTasks[key] {
			continue
		}
		mg.dupTasks[key] = true
		mg.writeTaskConnection(node, child)
		mg.writeTask(child)
	}
}

func (mg *MermaidGenerator) writeTaskConnection(from *Node, to *Node) {
	fromId := mg.sanitizeNodeId(from.task.TaskType)
	toId := mg.sanitizeNodeId(to.task.TaskType)

	var connector string
	if to.boldArrow {
		connector = " ==> " // Thick arrow for executed paths
	} else if mg.jobExecution == nil {
		connector = " --> " // Regular arrow when no execution data
	} else {
		connector = " -.-> " // Dotted arrow for potential but not-executed paths
	}

	if from.task.Method == common.AwaitForkedJob && to.task.Method == "ForkedJob" {
		mg.write(fmt.Sprintf("    %s%s%s\n", toId, connector, fromId))
	} else {
		mg.write(fmt.Sprintf("    %s%s%s\n", fromId, connector, toId))
	}
}

func (mg *MermaidGenerator) writeNode(node *Node) {
	nodeId := mg.sanitizeNodeId(node.task.TaskType)
	//label := node.task.TaskType

	// Choose shape based on task type and properties
	shape := mg.getNodeShape(node)
	mg.write(fmt.Sprintf("    %s%s\n", nodeId, shape))

	// Add styling
	color := mg.getNodeColor(node)
	mg.addNodeStyle(nodeId, color, node.bold)
}

func (mg *MermaidGenerator) getNodeShape(node *Node) string {
	label := node.task.TaskType

	if node.decision {
		return fmt.Sprintf("{\"%s\"}", label) // Diamond
	} else if node.task.AlwaysRun {
		return fmt.Sprintf("[/\"%s\"\\]", label) // Trapezoid
	} else if node.task.Method == common.ForkJob {
		return fmt.Sprintf("[\\\"%s\"/]", label) // Inverted trapezoid
	} else if node.task.Method == common.AwaitForkedJob {
		return fmt.Sprintf("[/\"%s\"\\]", label) // Trapezoid
	} else if node.task.Method == "ForkedJob" {
		return fmt.Sprintf("((\"%s\"))", label) // Circle
	} else if node.task.Method == common.Manual {
		return fmt.Sprintf("[\"%s\n🔒\"]", label) // Manual approval icon
	} else {
		return fmt.Sprintf("[\"%s\"]", label) // Rectangle
	}
}

func (mg *MermaidGenerator) getNodeColor(node *Node) string {
	if mg.jobExecution == nil {
		return defaultColor
	}

	switch node.state {
	case common.COMPLETED:
		return endSuccessColor
	case common.FAILED:
		return endFailColor
	case common.EXECUTING, common.READY:
		return processingColor
	case common.MANUAL_APPROVAL_REQUIRED:
		return "info"
	case common.PAUSED:
		return "warning"
	case common.CANCELLED:
		return "secondary"
	default:
		return defaultColor
	}
}

func (mg *MermaidGenerator) addNodeStyle(nodeId, color string, bold bool) {
	className := fmt.Sprintf("class_%s", color)
	if bold {
		className += "_bold"
	}
	mg.nodeStyles[nodeId] = className
}

func (mg *MermaidGenerator) writeStyles() {
	// Write class definitions
	mg.write("\n")

	// Define CSS classes
	classMap := map[string]string{
		"primary":   "fill:#e1f5fe,stroke:#01579b,stroke-width:2px",
		"success":   "fill:#e8f5e8,stroke:#2e7d32,stroke-width:2px",
		"error":     "fill:#ffebee,stroke:#c62828,stroke-width:2px",
		"warning":   "fill:#fff3e0,stroke:#ef6c00,stroke-width:2px",
		"info":      "fill:#e3f2fd,stroke:#1565c0,stroke-width:2px",
		"secondary": "fill:#f5f5f5,stroke:#616161,stroke-width:2px",
		"default":   "fill:#fafafa,stroke:#9e9e9e,stroke-width:1px",
	}

	boldMap := map[string]string{
		"primary_bold":   "fill:#e1f5fe,stroke:#01579b,stroke-width:4px",
		"success_bold":   "fill:#e8f5e8,stroke:#2e7d32,stroke-width:4px",
		"error_bold":     "fill:#ffebee,stroke:#c62828,stroke-width:4px",
		"warning_bold":   "fill:#fff3e0,stroke:#ef6c00,stroke-width:4px",
		"info_bold":      "fill:#e3f2fd,stroke:#1565c0,stroke-width:4px",
		"secondary_bold": "fill:#f5f5f5,stroke:#616161,stroke-width:4px",
		"default_bold":   "fill:#fafafa,stroke:#9e9e9e,stroke-width:3px",
	}

	// Combine maps
	allClasses := make(map[string]string)
	for k, v := range classMap {
		allClasses["class_"+k] = v
	}
	for k, v := range boldMap {
		allClasses["class_"+k] = v
	}

	// Write class definitions
	for className, style := range allClasses {
		mg.write(fmt.Sprintf("    classDef %s %s\n", className, style))
	}

	// Apply classes to nodes
	mg.write("\n")
	for nodeId, className := range mg.nodeStyles {
		mg.write(fmt.Sprintf("    class %s %s\n", nodeId, className))
	}
}

func (mg *MermaidGenerator) write(line string) {
	mg.buf.WriteString(line)
}

func (mg *MermaidGenerator) sanitizeNodeId(taskType string) string {
	// Replace characters that might cause issues in Mermaid
	id := strings.ReplaceAll(taskType, "-", "_")
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, ".", "_")
	return id
}

func (mg *MermaidGenerator) buildTree() (node *Node, nodes map[string]*Node, err error) {
	var firstTask *types.TaskDefinition
	firstTask, err = mg.jobDefinition.GetFirstTask()
	if err != nil {
		return
	}
	nodes = make(map[string]*Node)
	_, arrowColor, _ := mg.getTaskStateStateColor(firstTask.TaskType)
	node = &Node{task: firstTask, arrowColor: arrowColor}
	mg.addNodes(node, nodes)

	// Ensure all tasks in the job definition are included in nodes map
	// This handles cases where tasks might not be reachable through normal traversal
	for _, task := range mg.jobDefinition.Tasks {
		if _, exists := nodes[task.TaskType]; !exists {
			_, arrowColor, _ := mg.getTaskStateStateColor(task.TaskType)
			taskNode := &Node{task: task, arrowColor: arrowColor}
			mg.addNodes(taskNode, nodes)
		}
	}

	return
}

func (mg *MermaidGenerator) addNodes(parentNode *Node, nodes map[string]*Node) {
	if parentNode == nil || parentNode.task == nil {
		return
	}
	params := mg.jobDefinition.GetDynamicConfigAndVariables(nil)
	params["JobRetry"] = common.NewVariableValue(0, false)
	params["TaskRetry"] = common.NewVariableValue(0, false)
	params["Nonce"] = common.NewVariableValue(0, false)
	params["JobElapsedSecs"] = common.NewVariableValue(0, false)

	var fromExecTask *types.TaskExecution
	if mg.jobExecution != nil {
		_, fromExecTask = mg.jobExecution.GetTask("", parentNode.task.TaskType)
		for _, c := range mg.jobExecution.Contexts {
			if vv, err := c.GetVariableValue(); err == nil {
				params[c.Name] = vv
			}
		}
	}
	nodes[parentNode.task.TaskType] = parentNode
	parentNode.state, parentNode.color, parentNode.bold = mg.getTaskStateStateColor(parentNode.task.TaskType)

	if dynTask, _, err := mg.jobDefinition.GetDynamicTask(
		parentNode.task.TaskType,
		params); err == nil {
		parentNode.task = dynTask
	}

	if parentNode.task.Method == common.ForkJob {
		childTask := types.NewTaskDefinition(parentNode.task.ForkJobType, "ForkedJob")
		childNode := &Node{task: childTask, color: "gray", arrow: common.RequestState("fork"), arrowColor: parentNode.state.DotColor()}
		childNode.boldArrow = fromExecTask != nil && fromExecTask.TaskState.IsTerminal()
		parentNode.children = append(parentNode.children, childNode)
		nodes[childTask.TaskType] = childNode
	} else if parentNode.task.Method == common.AwaitForkedJob {
		for _, c := range parentNode.task.AwaitForkedTasks {
			forkedNode := nodes[c]
			if forkedNode != nil {
				childTask := types.NewTaskDefinition(forkedNode.task.ForkJobType, "ForkedJob")
				childNode := &Node{task: childTask, color: "gray", arrow: "await", arrowColor: parentNode.state.DotColor()}
				if fromExecTask != nil {
					childNode.boldArrow = fromExecTask.TaskState.IsTerminal()
					childNode.state = fromExecTask.TaskState
					childNode.bold = fromExecTask.TaskState.IsTerminal()
					childNode.color = fromExecTask.TaskState.DotColor()
				}
				parentNode.children = append(parentNode.children, childNode)
				nodes[childTask.TaskType] = childNode
			}
		}
	}

	for state, target := range parentNode.task.OnExitCode {
		childTask := mg.jobDefinition.GetTask(target)
		if childTask == nil {
			continue
		}
		childNode := &Node{task: childTask, arrow: state, arrowColor: state.DotColor()}
		if fromExecTask != nil {
			var nextTask *types.TaskDefinition
			parentNode.arrowColor = fromExecTask.TaskState.DotColor()
			nextTask, parentNode.decision, _ = mg.jobDefinition.GetNextTask(
				parentNode.task,
				fromExecTask.TaskState,
				fromExecTask.ExitCode)
			if nextTask != nil {
				childNode.boldArrow = fromExecTask.TaskState == parentNode.state && nextTask.TaskType == childNode.task.TaskType
			} else {
				if childTask != nil && childTask.AlwaysRun {
					matched := false
					for _, childTarget := range childTask.OnExitCode {
						_, childTargetExec := mg.jobExecution.GetTask("", childTarget)
						if childTargetExec != nil {
							matched = true
							break
						}
					}
					if !matched {
						childNode.boldArrow = true
						if fromExecTask.ExitCode != "" {
							childNode.arrow = common.RequestState(fromExecTask.ExitCode)
						} else {
							childNode.arrow = fromExecTask.TaskState
						}
					}
				}
			}
		}
		mg.addNodes(childNode, nodes)
		parentNode.children = append(parentNode.children, childNode)
	}
}

func (mg *MermaidGenerator) getTaskStateStateColor(taskType string) (common.RequestState, string, bool) {
	if mg.jobExecution == nil {
		return common.UNKNOWN, defaultColor, false
	}
	_, task := mg.jobExecution.GetTask("", taskType)
	if task == nil {
		return common.UNKNOWN, defaultColor, false
	}
	return task.TaskState, task.TaskState.DotColor(), !task.TaskState.Processing()
}
