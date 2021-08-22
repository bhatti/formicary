package dot

import (
	"bytes"
	"fmt"
	"github.com/goccy/go-graphviz"
	"strings"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

const (
	successColor    = "darkseagreen4"
	endSuccessColor = "darkseagreen"
	failColor       = "firebrick4"
	endFailColor    = "firebrick"
	blueColor       = "dodgerblue3"
	unknownColor    = "goldenrod3"
	executingColor  = "skyblue2"
	defaultColor    = "gray"
)

// Generator structure
type Generator struct {
	jobDefinition   *types.JobDefinition
	jobExecution    *types.JobExecution
	executionStatus common.RequestState
	dupTasks        map[string]bool
	buf             strings.Builder
}

// Node for building task graph
type Node struct {
	label      string
	state      common.RequestState
	color      string
	bold       bool
	arrow      common.RequestState
	arrowColor string
	boldArrow  bool
	decision   bool
	alwaysRun bool
	children   []*Node
}

// New constructor
func New(
	jobDefinition *types.JobDefinition,
	jobExecution *types.JobExecution) (*Generator, error) {
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
	return &Generator{
		jobDefinition:   jobDefinition,
		jobExecution:    jobExecution,
		executionStatus: executionStatus,
		dupTasks:        make(map[string]bool),
	}, nil
}

// GenerateDotImage creates dot image in PNG format
func (dg *Generator) GenerateDotImage() ([]byte, error) {
	g := graphviz.New()
	conf, err := dg.GenerateDot()
	if err != nil {
		return nil, err
	}
	graph, err := graphviz.ParseBytes([]byte(conf))
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	err = g.Render(graph, graphviz.PNG, &b)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// GenerateDot creates dot config
func (dg *Generator) GenerateDot() (string, error) {
	endColor := blueColor
	if dg.executionStatus.Completed() {
		endColor = endSuccessColor
	} else if dg.executionStatus.Failed() {
		endColor = endFailColor
	}
	head, nodes, err := dg.buildTree()
	if err != nil {
		return "", err
	}
	// writing DOT jobDefinition
	dg.write("digraph {\n")

	// write shapes of boxes
	if dg.jobExecution != nil {
		dg.write(fmt.Sprintf(`  "start" [shape=Mdiamond,color=%s,penwidth=3.0,style=solid,label="START"]`,
			blueColor) + "\n")
		dg.write(fmt.Sprintf(`  "end" [shape=Msquare,color=%s,penwidth=3.0,style=solid,label="END"]`,
			endColor) + "\n")
	}
	for _, node := range nodes {
		dg.writeShape(node)
	}

	if dg.jobExecution != nil {
		// write arrows for direction
		dg.write(fmt.Sprintf(`  "start" -> "%s" [color=%s,penwidth=3.0,style=solid,label="begin"];`+"\n",
			head.label, blueColor))
	}

	// writeTask will call recursively call all tasks in the job
	dg.writeTask(head)

	if dg.jobExecution != nil {
		lastType := "last"
		lastExecTask := dg.jobExecution.GetLastExecutedTask()
		if lastExecTask == nil {
			lastTask := dg.jobDefinition.GetLastTask()
			if lastTask != nil {
				lastType = lastTask.TaskType
			}
		} else {
			lastType = lastExecTask.TaskType
		}
		if endColor == blueColor {
			dg.write(fmt.Sprintf(`  "%s" -> "end" [color=%s,penwidth=1.0,style=dotted,label="finish"];`+"\n",
				lastType, endColor))
		} else {
			dg.write(fmt.Sprintf(`  "%s" -> "end" [color=%s,penwidth=3.0,style=solid,label="finish"];`+"\n",
				lastType, endColor))
		}
	}

	dg.write("}\n")
	return dg.buf.String(), nil
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (dg *Generator) writeTask(node *Node) {
	if node == nil {
		return
	}
	for _, child := range node.children {
		key := toKey(node.label, child.label)
		if dg.dupTasks[key] {
			continue
		}
		dg.dupTasks[key] = true
		dg.writeTaskLine(node, child)
		dg.writeTask(child)
	}
}

func (dg *Generator) writeTaskLine(
	from *Node,
	to *Node,
) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`  "%s" -> "%s" `, from.label, to.label))

	var widthStyle string
	if to.boldArrow {
		widthStyle = "penwidth=5.0,style=solid"
	} else {
		widthStyle = "penwidth=1.0,style=dotted"
	}

	sb.WriteString(fmt.Sprintf("[color=%s,%s,label=\"%s\"];\n",
		from.arrowColor, widthStyle, strings.ToLower(string(to.arrow))))
	dg.write(sb.String())
}

func (dg *Generator) write(line string) {
	dg.buf.WriteString(line)
}

func (dg *Generator) writeShape(node *Node) {
	dg.write(fmt.Sprintf(`  "%s" [shape=`, node.label))
	if dg.jobExecution == nil {
		dg.write("ellipse,color=gray,style=rounded]\n")
	} else {
		pendwidth := 2
		if !node.state.Processing() {
			pendwidth = 4
		}
		shape := "box"
		if node.decision {
			shape = "diamond"
		} else if node.alwaysRun {
			shape = "trapezium"
		}
		dg.write(fmt.Sprintf("%s,color=%s,style=rounded,penwidth=%d,pad=0.1]\n",
			shape, node.color, pendwidth))
	}
}

func (dg *Generator) buildTree() (node *Node, nodes map[string]*Node, err error) {
	var firstTask *types.TaskDefinition
	firstTask, err = dg.jobDefinition.GetFirstTask()
	if err != nil {
		return
	}
	nodes = make(map[string]*Node)
	_, arrowColor, _ := dg.getTaskStateStateColor(firstTask.TaskType)
	node = &Node{label: firstTask.TaskType, arrowColor: arrowColor, alwaysRun: firstTask.AlwaysRun}
	dg.addNodes(node, firstTask, nodes)
	return
}

func (dg *Generator) addNodes(parentNode *Node, parentTask *types.TaskDefinition, nodes map[string]*Node) {
	if parentNode == nil || parentTask == nil {
		return
	}
	nodes[parentTask.TaskType] = parentNode
	parentNode.state, parentNode.color, parentNode.bold = dg.getTaskStateStateColor(parentTask.TaskType)
	for state, target := range parentTask.OnExitCode {
		childTask := dg.jobDefinition.GetTask(target)
		if childTask == nil {
			continue
		}
		childNode := &Node{label: target, arrow: state, arrowColor: stateToColor(state), alwaysRun: childTask.AlwaysRun}
		if dg.jobExecution != nil {
			fromExecTask := dg.jobExecution.GetTask(parentTask.TaskType)
			if fromExecTask != nil {
				var nextTask *types.TaskDefinition
				parentNode.arrowColor = stateToColor(fromExecTask.TaskState)
				nextTask, parentNode.decision, _ = dg.jobDefinition.GetNextTask(
					parentTask,
					fromExecTask.TaskState,
					fromExecTask.ExitCode)
				if nextTask != nil {
					childNode.boldArrow = fromExecTask.TaskState == parentNode.state && nextTask.TaskType == childNode.label
				} else {
					if childTask != nil && childTask.AlwaysRun {
						matched := false
						for _, childTarget := range childTask.OnExitCode {
							childTargetExec := dg.jobExecution.GetTask(childTarget)
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
			dg.addNodes(childNode, childTask, nodes)
			parentNode.children = append(parentNode.children, childNode)
		}
	}
}

func (dg *Generator) getTaskStateStateColor(taskType string) (common.RequestState, string, bool) {
	if dg.jobExecution == nil {
		return common.UNKNOWN, defaultColor, false
	}
	task := dg.jobExecution.GetTask(taskType)
	if task == nil {
		return common.UNKNOWN, defaultColor, false
	}
	return task.TaskState, stateToColor(task.TaskState), !task.TaskState.Processing()
}

func stateToColor(state common.RequestState) string {
	if state.Completed() {
		return successColor
	} else if state.Failed() {
		return failColor
	} else if state.Executing() {
		return executingColor
	} else if state.Unknown() {
		return unknownColor
	} else {
		return defaultColor
	}
}

func toKey(from string, to string) string {
	return from + ":" + to
}
