package diagrams

import (
	"bytes"
	"fmt"
	"github.com/goccy/go-graphviz"
	"strings"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

// DotGenerator structure
type DotGenerator struct {
	jobDefinition   *types.JobDefinition
	jobExecution    *types.JobExecution
	executionStatus common.RequestState
	dupTasks        map[string]bool
	buf             strings.Builder
}

// Node for building task graph
type Node struct {
	task       *types.TaskDefinition
	state      common.RequestState
	color      string
	bold       bool
	arrow      common.RequestState
	arrowColor string
	boldArrow  bool
	decision   bool
	children   []*Node
}

// NewDot constructor
func NewDot(
	jobDefinition *types.JobDefinition,
	jobExecution *types.JobExecution) (*DotGenerator, error) {
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
	return &DotGenerator{
		jobDefinition:   jobDefinition,
		jobExecution:    jobExecution,
		executionStatus: executionStatus,
		dupTasks:        make(map[string]bool),
	}, nil
}

// GenerateDotImage creates diagrams image in PNG format
func (dg *DotGenerator) GenerateDotImage() ([]byte, error) {
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

// GenerateDot creates diagrams config
func (dg *DotGenerator) GenerateDot() (string, error) {
	const (
		endSuccessColor = "darkseagreen"
		endFailColor    = "firebrick"
		blueColor       = "dodgerblue3"
		defaultColor    = "gray"
	)

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
		if dg.jobExecution.JobState.IsTerminal() {
			dg.write(fmt.Sprintf(`  "end" [shape=Msquare,color=%s,penwidth=3.0,style=solid,label="END"]`,
				endColor) + "\n")
		}
	}
	for _, node := range nodes {
		dg.writeShape(node)
	}

	if dg.jobExecution != nil {
		// write arrows for direction
		dg.write(fmt.Sprintf(`  "start" -> "%s" [color=%s,penwidth=3.0,style=solid,label="begin"];`+"\n",
			head.task.TaskType, blueColor))
	}

	// writeTask will call recursively call all tasks in the job
	dg.writeTask(head)

	if dg.jobExecution != nil && dg.jobExecution.JobState.IsTerminal() {
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

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (dg *DotGenerator) writeTask(node *Node) {
	if node == nil {
		return
	}
	for _, child := range node.children {
		key := toKey(node.task.TaskType, child.task.TaskType)
		if dg.dupTasks[key] {
			continue
		}
		dg.dupTasks[key] = true
		dg.writeTaskLine(node, child)
		dg.writeTask(child)
	}
}

func (dg *DotGenerator) writeTaskLine(
	from *Node,
	to *Node,
) {
	var sb strings.Builder
	if from.task.Method == common.AwaitForkedJob && to.task.Method == "ForkedJob" {
		sb.WriteString(fmt.Sprintf(`  "%s" -> "%s" `, to.task.TaskType, from.task.TaskType))
	} else {
		sb.WriteString(fmt.Sprintf(`  "%s" -> "%s" `, from.task.TaskType, to.task.TaskType))
	}

	var arrowColor string
	var widthStyle string
	if to.boldArrow {
		widthStyle = "penwidth=5.0,style=solid"
		arrowColor = from.arrowColor
	} else {
		widthStyle = "penwidth=1.0,style=dotted"
		arrowColor = "gray"
	}

	sb.WriteString(fmt.Sprintf("[color=%s,%s,label=\"%s\"];\n",
		arrowColor, widthStyle, strings.ToLower(string(to.arrow))))
	dg.write(sb.String())
}

func (dg *DotGenerator) write(line string) {
	dg.buf.WriteString(line)
}

func (dg *DotGenerator) writeShape(node *Node) {
	dg.write(fmt.Sprintf(`  "%s" [shape=`, node.task.TaskType))
	if dg.jobExecution == nil {
		shape := overriddenShape(node, "ellipse")
		dg.write(fmt.Sprintf("%s,color=gray,style=rounded]\n", shape))
	} else {
		pendwidth := 2
		if !node.state.Processing() {
			pendwidth = 4
		}
		shape := overriddenShape(node, "box")
		dg.write(fmt.Sprintf("%s,color=%s,style=rounded,penwidth=%d,pad=0.1]\n",
			shape, node.color, pendwidth))
	}
}

func overriddenShape(node *Node, shape string) string {
	if node.decision {
		shape = "diamond"
	} else if node.task.AlwaysRun {
		shape = "trapezium"
	} else if node.task.Method == common.ForkJob {
		shape = "invhouse"
	} else if node.task.Method == common.AwaitForkedJob {
		shape = "house"
	} else if node.task.Method == common.Manual {
		shape = "circle"
	} else if node.task.Method == "ForkedJob" {
		shape = "component"
	}
	return shape
}

func (dg *DotGenerator) buildTree() (node *Node, nodes map[string]*Node, err error) {
	var firstTask *types.TaskDefinition
	firstTask, err = dg.jobDefinition.GetFirstTask()
	if err != nil {
		return
	}
	nodes = make(map[string]*Node)
	_, arrowColor, _ := dg.getTaskStateStateColor(firstTask.TaskType)
	node = &Node{task: firstTask, arrowColor: arrowColor}
	dg.addNodes(node, nodes)
	return
}

func (dg *DotGenerator) addNodes(parentNode *Node, nodes map[string]*Node) {
	if parentNode == nil || parentNode.task == nil {
		return
	}
	params := dg.jobDefinition.GetDynamicConfigAndVariables(nil)
	params["JobRetry"] = common.NewVariableValue(0, false)
	params["TaskRetry"] = common.NewVariableValue(0, false)
	params["Nonce"] = common.NewVariableValue(0, false)
	params["JobElapsedSecs"] = common.NewVariableValue(0, false)

	var fromExecTask *types.TaskExecution
	if dg.jobExecution != nil {
		_, fromExecTask = dg.jobExecution.GetTask("", parentNode.task.TaskType)
		for _, c := range dg.jobExecution.Contexts {
			if vv, err := c.GetVariableValue(); err == nil {
				params[c.Name] = vv
			}
		}
	}
	nodes[parentNode.task.TaskType] = parentNode
	parentNode.state, parentNode.color, parentNode.bold = dg.getTaskStateStateColor(parentNode.task.TaskType)

	if dynTask, _, err := dg.jobDefinition.GetDynamicTask(
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
		childTask := dg.jobDefinition.GetTask(target)
		if childTask == nil {
			continue
		}
		childNode := &Node{task: childTask, arrow: state, arrowColor: state.DotColor()}
		if fromExecTask != nil {
			var nextTask *types.TaskDefinition
			parentNode.arrowColor = fromExecTask.TaskState.DotColor()
			nextTask, parentNode.decision, _ = dg.jobDefinition.GetNextTask(
				parentNode.task,
				fromExecTask.TaskState,
				fromExecTask.ExitCode)
			if nextTask != nil {
				childNode.boldArrow = fromExecTask.TaskState == parentNode.state && nextTask.TaskType == childNode.task.TaskType
			} else {
				if childTask != nil && childTask.AlwaysRun {
					matched := false
					for _, childTarget := range childTask.OnExitCode {
						_, childTargetExec := dg.jobExecution.GetTask("", childTarget)
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
		dg.addNodes(childNode, nodes)
		parentNode.children = append(parentNode.children, childNode)
	}
}

func (dg *DotGenerator) getTaskStateStateColor(taskType string) (common.RequestState, string, bool) {
	if dg.jobExecution == nil {
		return common.UNKNOWN, defaultColor, false
	}
	_, task := dg.jobExecution.GetTask("", taskType)
	if task == nil {
		return common.UNKNOWN, defaultColor, false
	}
	return task.TaskState, task.TaskState.DotColor(), !task.TaskState.Processing()
}

func toKey(from string, to string) string {
	return from + ":" + to
}
