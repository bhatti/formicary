package dot

import (
	"bytes"
	"fmt"
	"github.com/goccy/go-graphviz"
	"strings"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

const successColor = "darkseagreen4"
const failColor = "firebrick4"
const blueColor = "dodgerblue3"
const unknownColor = "goldenrod3"

// Generator structure
type Generator struct {
	definition      *types.JobDefinition
	executionStatus common.RequestState
	lastTaskTypes   []string
	statuses        map[string]common.RequestState
	dupFromToStatus map[string]bool
	dupTasks        map[string]bool
	buf             strings.Builder
}

// New constructor
func New(
	definition *types.JobDefinition,
	execution *types.JobExecution) (*Generator, error) {
	if definition == nil {
		return nil, fmt.Errorf("job definition is not specified")
	}
	executionStatus := common.PENDING
	statuses := make(map[string]common.RequestState)
	if execution != nil {
		executionStatus = execution.JobState
		for _, t := range execution.Tasks {
			statuses[t.TaskType] = common.NewRequestState(string(t.TaskState))
		}
	}
	lastTasks := definition.GetLastAlwaysRunTasks()
	var lastTaskTypes []string
	if len(lastTasks) > 0 {
		for _, lastTask := range lastTasks {
			lastTaskTypes = append(lastTaskTypes, lastTask.TaskType)
		}
	} else {
		lastTaskTypes = []string{"end"}
	}
	return &Generator{
		definition:      definition,
		executionStatus: executionStatus,
		lastTaskTypes:   lastTaskTypes,
		statuses:        statuses,
		dupFromToStatus: make(map[string]bool),
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
	firstTask, err := dg.definition.GetFirstTask()
	if err != nil {
		return "", err
	}
	endColor := blueColor
	if dg.executionStatus.Completed() {
		endColor = successColor
	} else if dg.executionStatus.Failed() {
		endColor = failColor
	}
	// writing DOT definition
	dg.write("digraph {\n")

	lastTask := dg.definition.GetLastTask()

	// write shapes of boxes
	if dg.executionStatus != common.PENDING {
		dg.write(fmt.Sprintf(`  "start" [shape=Mdiamond,color=%s,penwidth=3.0,style=solid,label="START"]`,
			blueColor) + "\n")
		dg.write(fmt.Sprintf(`  "end" [shape=Msquare,color=%s,penwidth=3.0,style=solid,label="END"]`,
			endColor) + "\n")
	}
	for _, t := range dg.definition.Tasks {
		dg.write(getShape(t, dg.statuses))
	}

	if dg.executionStatus != common.PENDING {
		// write arrows for direction
		dg.write(fmt.Sprintf(`  "start" -> "%s" [color=%s,penwidth=3.0,style=solid,label="begin"];`+"\n",
			firstTask.TaskType, blueColor))
	}

	// writeTask will call recursively call all tasks in the job
	dg.writeTask(firstTask, " ")

	if dg.executionStatus != common.PENDING {
		key := toKey(lastTask.TaskType, "end", dg.executionStatus)
		//if lastExecTask := dg.execution.GetLastTask(); lastExecTask != nil && lastExecTask.Failed() {
		//	logrus.Infof("last %v , job status %v, color %v", lastExecTask.TaskType, dg.execution.JobState, endColor)
		//	dg.write(fmt.Sprintf(`  "%s" -> "end" [color=%s,penwidth=3.0,style=solid,label="%s"];`+"\n",
		//		lastExecTask.TaskType, endColor, strings.ToLower(string(dg.executionStatus))))
		//} else
		if lastTask != nil && dg.dupFromToStatus[key] == false {
			if dg.statuses[lastTask.TaskType] == "" {
				dg.write(fmt.Sprintf(`  "%s" -> "end" [color=%s,penwidth=1.0,style=dotted,label="finish"];`+"\n",
					lastTask.TaskType, endColor))
			} else {
				dg.write(fmt.Sprintf(`  "%s" -> "end" [color=%s,penwidth=3.0,style=solid,label="finish"];`+"\n",
					lastTask.TaskType, endColor))
			}
		}
	}

	dg.write("}\n")
	return dg.buf.String(), nil
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (dg *Generator) nextLastType(path string) string {
	for _, lastTask := range dg.lastTaskTypes {
		if !strings.Contains(path, fmt.Sprintf(" %s ", lastTask)) {
			return lastTask
		}
	}
	return "end"
}

func (dg *Generator) writeTask(task *types.TaskDefinition, path string) {
	if task == nil {
		return
	}
	if dg.dupTasks[task.TaskType] {
		//logrus.Debugf("duplicate task %s, status %v", task.TaskType, dg.statuses)
		return
	}
	dg.dupTasks[task.TaskType] = true

	exitCodes := make(map[common.RequestState]string)
	for status, target := range task.OnExitCode {
		status = common.NewRequestState(string(status))
		exitCodes[status] = target
	}
	if len(exitCodes) > 0 && exitCodes[common.FAILED] == "" {
		exitCodes[common.FAILED] = dg.nextLastType(path)
	}

	for status, target := range exitCodes {
		if target == task.TaskType {
			continue
		}
		status = common.NewRequestState(string(status))
		bold := dg.statuses[task.TaskType] == status
		key := toKey(task.TaskType, target, status)
		if dg.dupFromToStatus[key] == false {
			dg.write(getTaskLine(task.TaskType, target, status, bold))
			dg.dupFromToStatus[key] = true
		}
		dg.writeTask(dg.definition.GetTask(target), fmt.Sprintf("%s %s ", path, task.TaskType))
	}
}

func toKey(from string, to string, status common.RequestState) string {
	return from + ":" + to + ":" + string(status)
}

func getTaskLine(
	from string,
	to string,
	status common.RequestState,
	bold bool) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`  "%s" -> "%s" `, from, to))

	var widthStyle string
	if bold {
		widthStyle = "penwidth=5.0,style=solid"
	} else {
		widthStyle = "penwidth=1.0,style=dotted"
	}

	if status.Completed() {
		sb.WriteString("[color=darkseagreen4," + widthStyle)
	} else if status.Failed() {
		sb.WriteString(fmt.Sprintf("[color=%s,%s", failColor, widthStyle))
	} else if status.Executing() {
		sb.WriteString("[color=skyblue2," + widthStyle)
	} else if status.Unknown() {
		sb.WriteString(fmt.Sprintf("[color=%s,%s", unknownColor, widthStyle))
	} else {
		sb.WriteString("[color=gray," + widthStyle)
	}
	sb.WriteString(fmt.Sprintf(`,label="%s"]`, strings.ToLower(string(status))))
	sb.WriteString(";\n")
	return sb.String()
}

func getShape(
	task *types.TaskDefinition,
	statuses map[string]common.RequestState) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`  "%s" [shape=`, task.TaskType))
	status := statuses[task.TaskType]
	matches := status != common.PENDING && status != common.READY && status != ""

	if matches {
		sb.WriteString("box,color=")
		if status.Completed() {
			sb.WriteString("darkseagreen4,style=rounded,penwidth=4]\n")
		} else if status.Failed() {
			sb.WriteString(fmt.Sprintf("%s,style=rounded,penwidth=4]\n", failColor))
		} else if status.Executing() || status.Started() {
			sb.WriteString("darkorange,style=rounded,penwidth=2]\n")
		} else {
			sb.WriteString("skyblue1,style=rounded,penwidth=2]\n")
		}
	} else {
		sb.WriteString("ellipse,color=gray,style=rounded]\n")
	}
	return sb.String()
}

func (dg *Generator) write(line string) {
	dg.buf.WriteString(line)
}
