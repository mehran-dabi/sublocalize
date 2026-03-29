package output

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"sublocalize/internal/srt"
)

func WriteSRT(filename string, subs []srt.Subtitle, rtl bool) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for i, s := range subs {
		fmt.Fprintf(w, "%d\n", s.Index)
		fmt.Fprintf(w, "%s --> %s\n", srt.FormatTimestamp(s.Start), srt.FormatTimestamp(s.End))
		lines := strings.Split(s.Text, "\n")
		for _, line := range lines {
			if rtl {
				fmt.Fprintf(w, "\u202B%s\u202C\n", line)
			} else {
				fmt.Fprintln(w, line)
			}
		}
		if i < len(subs)-1 {
			fmt.Fprintln(w)
		}
	}
	return w.Flush()
}
