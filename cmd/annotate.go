package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mincong-classroom/mc/annotate"
)

var (
	annotateAll       bool
	annotateEvaluator bool
	annotatePadding   int
)

var annotateCmd = &cobra.Command{
	Use:   "annotate",
	Short: "Find a text in a screenshot, then draw numbered boxes around what the evaluator found",
	Long: `Find a text in a screenshot, then copy the screenshot with numbered boxes around what the
evaluator found: a green box validates something, a red box is a flag. The text next to the
image refers to each box by its number.`,
	Example: `  mc annotate find build.png "BUILD SUCCESS"
  mc annotate draw build.png build-boxed.png 120,40,300,22,green,1 40,90,210,22,red,2`,
}

var annotateFindCmd = &cobra.Command{
	Use:   "find IMAGE [TEXT]",
	Short: "Print the box of each line of a screenshot that contains a text",
	Long: `Print the box of each line of the screenshot that contains TEXT: x,y,w,h, then the line as
tesseract read it. The search ignores the case, the spaces and the kind of dash, since tesseract
often splits a word in two and reads a long dash as a hyphen. --all prints every line instead.
It needs tesseract.`,
	Args:         cobra.RangeArgs(1, 2),
	RunE:         runAnnotateFind,
	SilenceUsage: true,
}

var annotateDrawCmd = &cobra.Command{
	Use:   "draw IMAGE OUT BOX...",
	Short: "Copy a screenshot with a numbered box around each BOX",
	Long: `Copy IMAGE to OUT, a PNG file, with a box around each BOX, written x,y,w,h,COLOR,LABEL: the box
as find prints it, COLOR green when the box validates something and red when it is a flag, and
LABEL its number. The box has a thick border, a padding around the text and no fill, and its
number is in a circle of its color, on its right side: the image is widened when a box ends too
close to its right edge.

A gray band at the bottom says that the screenshot is the team's, and that the evaluator added
the boxes. --evaluator leaves it out, for a screenshot that already carries the watermark of the
evaluator.`,
	Args:         cobra.MinimumNArgs(3),
	RunE:         runAnnotateDraw,
	SilenceUsage: true,
}

func init() {
	annotateFindCmd.Flags().BoolVar(&annotateAll, "all", false, "Print every line of the screenshot")
	annotateDrawCmd.Flags().BoolVar(&annotateEvaluator, "evaluator", false, "A screenshot of the evaluator: no band at the bottom")
	annotateDrawCmd.Flags().IntVar(&annotatePadding, "padding", 0, "The space between a text and its box, in pixels (default: the width of the border)")
	annotateCmd.AddCommand(annotateFindCmd, annotateDrawCmd)
	RootCmd.AddCommand(annotateCmd)
}

func runAnnotateFind(cmd *cobra.Command, args []string) error {
	if len(args) < 2 && !annotateAll {
		return errors.New("a text to find, or --all")
	}
	img, err := annotate.Load(args[0])
	if err != nil {
		return err
	}
	lines, err := annotate.Read(cmd.Context(), img)
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	if annotateAll {
		for _, l := range lines {
			fmt.Fprintf(w, "%s\t%s\n", l.Box(), l.Text())
		}
		return nil
	}
	found := false
	for _, l := range lines {
		if b, ok := l.Find(args[1]); ok {
			fmt.Fprintf(w, "%s\t%s\n", b, l.Text())
			found = true
		}
	}
	if !found {
		return fmt.Errorf("%q not found: try a shorter text, or mc annotate find %s --all", args[1], args[0])
	}
	return nil
}

func runAnnotateDraw(_ *cobra.Command, args []string) error {
	marks := make([]annotate.Mark, 0, len(args)-2)
	for _, spec := range args[2:] {
		m, err := annotate.ParseMark(spec)
		if err != nil {
			return err
		}
		marks = append(marks, m)
	}
	if annotatePadding < 0 {
		return fmt.Errorf("bad padding %d: 0 or more pixels", annotatePadding)
	}
	img, err := annotate.Load(args[0])
	if err != nil {
		return err
	}
	opts := annotate.Options{Padding: annotatePadding}
	if !annotateEvaluator {
		opts.Band = "Screenshot of the team. The numbered boxes were added by the evaluator on " +
			time.Now().Format("Mon 2 Jan 2006") + "."
	}
	out, err := annotate.Draw(img, marks, opts)
	if err != nil {
		return err
	}
	return annotate.Save(args[1], out)
}
