package reports

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/bridgecrewio/yor/src/common"
	"github.com/bridgecrewio/yor/src/common/logger"
	"github.com/bridgecrewio/yor/src/common/tagging/tags"
	"github.com/olekukonko/tablewriter"
)

type ReportService struct {
	report Report
}

type ReportSummary struct {
	Scanned          int `json:"scanned"`
	NewResources     int `json:"newResources"`
	UpdatedResources int `json:"updatedResources"`
}

type TagRecord struct {
	File         string `json:"file"`
	ResourceID   string `json:"resourceId"`
	TagKey       string `json:"key"`
	OldValue     string `json:"oldValue"`
	UpdatedValue string `json:"updatedValue"`
	YorTraceID   string `json:"yorTraceId"`
}

type Report struct {
	Summary             ReportSummary `json:"summary"`
	NewResourceTags     []TagRecord   `json:"newResourceTags"`
	UpdatedResourceTags []TagRecord   `json:"updatedResourceTags"`
}

func (r *Report) AsJSONBytes() ([]byte, error) {
	jr, err := json.MarshalIndent(r, "", "    ")
	if err != nil {
		return nil, err
	}
	return jr, nil
}

var ReportServiceInst *ReportService

func init() {
	ReportServiceInst = &ReportService{}
}

func (r *ReportService) GetReport() *Report {
	return &r.report
}

func (r *ReportService) CreateReport() *Report {
	changesAccumulator := TagChangeAccumulatorInstance
	r.report.Summary = ReportSummary{
		Scanned:          len(changesAccumulator.ScannedBlocks),
		NewResources:     len(changesAccumulator.NewBlockTraces),
		UpdatedResources: len(changesAccumulator.UpdatedBlockTraces),
	}
	r.report.NewResourceTags = []TagRecord{}
	for _, block := range changesAccumulator.NewBlockTraces {
		for _, tag := range block.GetNewTags() {
			r.report.NewResourceTags = append(r.report.NewResourceTags, TagRecord{
				File:         block.GetFilePath(),
				ResourceID:   block.GetResourceID(),
				TagKey:       tag.GetKey(),
				OldValue:     "",
				UpdatedValue: tag.GetValue(),
				YorTraceID:   block.GetTraceID(),
			})
		}
	}
	r.report.UpdatedResourceTags = []TagRecord{}
	for _, block := range changesAccumulator.UpdatedBlockTraces {
		diff := block.CalculateTagsDiff()

		sort.SliceStable(diff.Added, func(i, j int) bool {
			return diff.Added[i].GetKey() < diff.Added[j].GetKey()
		})
		for _, val := range diff.Added {
			r.report.UpdatedResourceTags = append(r.report.UpdatedResourceTags, TagRecord{
				File:         block.GetFilePath(),
				ResourceID:   block.GetResourceID(),
				TagKey:       val.GetKey(),
				OldValue:     "",
				UpdatedValue: val.GetValue(),
				YorTraceID:   block.GetTraceID(),
			})
		}

		sort.SliceStable(diff.Updated, func(i, j int) bool {
			return diff.Updated[i].Key < diff.Updated[j].Key
		})
		for _, val := range diff.Updated {
			r.report.UpdatedResourceTags = append(r.report.UpdatedResourceTags, TagRecord{
				File:         block.GetFilePath(),
				ResourceID:   block.GetResourceID(),
				TagKey:       val.Key,
				OldValue:     val.PrevValue,
				UpdatedValue: val.NewValue,
				YorTraceID:   block.GetTraceID(),
			})
		}
	}
	return &r.report
}

// PrintToStdout prints the Report to the normal std::out. The structure:
// <Banner>
// Scanned Resources: <int>
// New Resources Traced: <int>
// Updated Resources: <int>
// <New Resources Table> as generated by printNewResourcesToStdout, if not empty
// <Updated Resources Table> as generated by printUpdatedResourcesToStdout, if not empty
func (r *ReportService) PrintToStdout(colors *common.ColorStruct) {
	PrintBanner(colors)
	fmt.Println(colors.Reset, "Yor Findings Summary")
	fmt.Println(colors.Reset, "Scanned Resources:\t", colors.Blue, r.report.Summary.Scanned)
	fmt.Println(colors.Reset, "New Resources Traced: \t", colors.Yellow, r.report.Summary.NewResources)
	fmt.Println(colors.Reset, "Updated Resources:\t", colors.Green, r.report.Summary.UpdatedResources)
	fmt.Println()
	if r.report.Summary.NewResources > 0 {
		r.printNewResourcesToStdout(colors)
	}
	fmt.Println()
	if r.report.Summary.UpdatedResources > 0 {
		r.printUpdatedResourcesToStdout(colors)
	}
}

func PrintBanner(colors *common.ColorStruct) {
	fmt.Printf("%v%vv%v\n", common.YorLogo, colors.Purple, common.Version)
}

func (r *ReportService) printUpdatedResourcesToStdout(colors *common.ColorStruct) {
	fmt.Print(colors.Green, fmt.Sprintf("Updated Resource Traces (%v):\n", r.report.Summary.UpdatedResources), colors.Reset)
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"File", "Resource", "Tag Key", "Old Value", "Updated Value", "Yor ID"})
        if !colors.NoColor {
	        table.SetColumnColor(
        		tablewriter.Colors{},
        		tablewriter.Colors{},
        		tablewriter.Colors{tablewriter.Bold},
        		tablewriter.Colors{tablewriter.Normal, tablewriter.FgRedColor},
        		tablewriter.Colors{tablewriter.Normal, tablewriter.FgGreenColor},
        		tablewriter.Colors{},
        	)
        }

	table.SetRowLine(true)
	table.SetRowSeparator("-")

	for _, tr := range r.report.UpdatedResourceTags {
		table.Append([]string{tr.File, tr.ResourceID, tr.TagKey, tr.OldValue, tr.UpdatedValue, tr.YorTraceID})
	}
	table.SetAutoMergeCellsByColumnIndex([]int{0, 1, 5})
	table.Render()
}

func (r *ReportService) printNewResourcesToStdout(colors *common.ColorStruct) {
	fmt.Print(colors.Yellow, fmt.Sprintf("New Resources Traced (%v):\n", r.report.Summary.NewResources), colors.Reset)
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"File", "Resource", "Tag Key", "Tag Value", "Yor ID"})
	table.SetRowLine(true)
	table.SetRowSeparator("-")
        if !colors.NoColor {
        	table.SetColumnColor(
        		tablewriter.Colors{},
        		tablewriter.Colors{},
        		tablewriter.Colors{tablewriter.Bold},
        		tablewriter.Colors{tablewriter.Normal, tablewriter.FgGreenColor},
        		tablewriter.Colors{},
	        )
        }
	for _, tr := range r.report.NewResourceTags {
		table.Append([]string{tr.File, tr.ResourceID, tr.TagKey, tr.UpdatedValue, tr.YorTraceID})
	}
	table.SetAutoMergeCellsByColumnIndex([]int{0, 1, 4})
	table.Render()
}

func (r *ReportService) PrintJSONToFile(file string) {
	jr, err := r.report.AsJSONBytes()
	if err != nil {
		logger.Warning("Failed to create report as JSON")
	}

	err = os.WriteFile(file, jr, 0600)
	if err != nil {
		logger.Warning("Failed to write to JSON file", err.Error())
	}
}

func (r *ReportService) PrintJSONToStdout() {
	jr, err := r.report.AsJSONBytes()
	if err != nil {
		logger.Error("couldn't parse report to JSON")
	}
	fmt.Println(string(jr))
}

func (r *ReportService) PrintTagGroupTags(tagsByGroup map[string][]tags.ITag) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Group", "Tag Key", "Description"})
	table.SetRowLine(true)
	table.SetRowSeparator("-")
	for group, groupTags := range tagsByGroup {
		if len(groupTags) > 0 {
			for _, tag := range groupTags {
				table.Append([]string{group, tag.GetKey(), tag.GetDescription()})
			}
		} else {
			table.Append([]string{group, "", ""})
		}
	}
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.Render()
}
