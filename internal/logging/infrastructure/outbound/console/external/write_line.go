package external

import "io"

// WriteLine writes the text followed by a line break with a single Write call, so lines written
// concurrently to the same output are not interleaved.
func WriteLine(writer io.Writer, line []byte) error {
	line_with_break := make([]byte, 0, len(line)+1)
	line_with_break = append(line_with_break, line...)
	line_with_break = append(line_with_break, '\n')

	_, err := writer.Write(line_with_break)
	return err
}
