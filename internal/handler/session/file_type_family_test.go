package session

import "testing"

func TestContainsFileTypeAdmitsTheOptionFamily(t *testing.T) {
	cases := []struct {
		supported []string
		ext       string
		want      bool
	}{
		{[]string{"csv", "xlsx"}, "xlsb", true},
		{[]string{"csv", "xlsx"}, "xls", true},
		{[]string{"csv", "xlsx"}, "csv", true},
		{[]string{"docx"}, "doc", true},
		{[]string{".JPG"}, "png", true},
		{[]string{"csv", "xlsx"}, "docx", false},
		{[]string{"xls"}, "xlsx", false},
		{[]string{"pdf"}, "doc", false},
	}
	for _, tc := range cases {
		if got := containsFileType(tc.supported, tc.ext); got != tc.want {
			t.Errorf("containsFileType(%v, %q) = %v, want %v", tc.supported, tc.ext, got, tc.want)
		}
	}
}
