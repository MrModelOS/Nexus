package internal

import (
	"fmt"
	"strings"
)

type DiffLine struct {
	Type    string
	Content string
	OldNum  int
	NewNum  int
}

type DiffHunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []DiffLine
}

type DiffFile struct {
	OldName string
	NewName string
	Hunks   []DiffHunk
}

type DiffViewer struct {
	Width   int
	Height  int
	Cursor  int
	Scroll  int
	File    *DiffFile
}

func NewDiffViewer(width, height int) *DiffViewer {
	return &DiffViewer{
		Width:  width,
		Height: height,
	}
}

func (d *DiffViewer) ParseDiff(diff string) *DiffFile {
	file := &DiffFile{}
	var currentHunk *DiffHunk

	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "--- ") {
			file.OldName = strings.TrimPrefix(line, "--- ")
		} else if strings.HasPrefix(line, "+++ ") {
			file.NewName = strings.TrimPrefix(line, "+++ ")
		} else if strings.HasPrefix(line, "@@") {
			if currentHunk != nil {
				file.Hunks = append(file.Hunks, *currentHunk)
			}
			currentHunk = &DiffHunk{}
			fmt.Sscanf(line, "@@ -%d +%d @@", &currentHunk.OldStart, &currentHunk.NewStart)
		} else if currentHunk != nil {
			diffLine := DiffLine{Content: line}
			if strings.HasPrefix(line, "+") {
				diffLine.Type = "add"
			} else if strings.HasPrefix(line, "-") {
				diffLine.Type = "del"
			} else {
				diffLine.Type = "ctx"
			}
			currentHunk.Lines = append(currentHunk.Lines, diffLine)
		}
	}

	if currentHunk != nil {
		file.Hunks = append(file.Hunks, *currentHunk)
	}

	d.File = file
	return file
}

func (d *DiffViewer) Render() string {
	if d.File == nil {
		return "No diff loaded."
	}

	var out strings.Builder

	header := fmt.Sprintf("\033[1;35mDiff:\033[0m %s → %s", d.File.OldName, d.File.NewName)
	out.WriteString(header)
	out.WriteString("\n\n")

	for _, hunk := range d.File.Hunks {
		out.WriteString(fmt.Sprintf("\033[1;36m@@ -%d +%d @@\033[0m\n", hunk.OldStart, hunk.NewStart))

		for _, line := range hunk.Lines {
			switch line.Type {
			case "add":
				out.WriteString(fmt.Sprintf("\033[32m+%s\033[0m\n", line.Content))
			case "del":
				out.WriteString(fmt.Sprintf("\033[31m-%s\033[0m\n", line.Content))
			default:
				out.WriteString(fmt.Sprintf(" %s\n", line.Content))
			}
		}
		out.WriteString("\n")
	}

	return out.String()
}

func (d *DiffViewer) RenderSideBySide() string {
	if d.File == nil {
		return "No diff loaded."
	}

	var out strings.Builder
	halfWidth := (d.Width - 3) / 2

	out.WriteString(fmt.Sprintf("\033[1;35mDiff:\033[0m %s → %s\n\n", d.File.OldName, d.File.NewName))

	for _, hunk := range d.File.Hunks {
		out.WriteString(fmt.Sprintf("\033[1;36m@@ -%d +%d @@\033[0m\n", hunk.OldStart, hunk.NewStart))

		var leftLines, rightLines []string

		for _, line := range hunk.Lines {
			switch line.Type {
			case "add":
				rightLines = append(rightLines, fmt.Sprintf("\033[32m+%s\033[0m", line.Content))
				leftLines = append(leftLines, strings.Repeat(" ", halfWidth))
			case "del":
				leftLines = append(leftLines, fmt.Sprintf("\033[31m-%s\033[0m", line.Content))
				rightLines = append(rightLines, strings.Repeat(" ", halfWidth))
			default:
				content := line.Content
				if len(content) > halfWidth {
					content = content[:halfWidth-3] + "..."
				}
				leftLines = append(leftLines, content)
				rightLines = append(rightLines, content)
			}
		}

		for i := 0; i < len(leftLines); i++ {
			left := leftLines[i]
			if len(left) < halfWidth {
				left += strings.Repeat(" ", halfWidth-len(left))
			}
			out.WriteString(fmt.Sprintf(" %s │ %s\n", left, rightLines[i]))
		}
		out.WriteString("\n")
	}

	return out.String()
}

func (d *DiffViewer) ScrollUp() {
	if d.Scroll > 0 {
		d.Scroll--
	}
}

func (d *DiffViewer) ScrollDown() {
	d.Scroll++
}

func (d *DiffViewer) Stats() (added, removed int) {
	if d.File == nil {
		return 0, 0
	}

	for _, hunk := range d.File.Hunks {
		for _, line := range hunk.Lines {
			if line.Type == "add" {
				added++
			} else if line.Type == "del" {
				removed++
			}
		}
	}

	return
}
