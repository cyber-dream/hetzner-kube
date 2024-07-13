package types

import "github.com/jedib0t/go-pretty/v6/table"

var TableStyle table.Style

func init() {
	TableStyle = table.StyleLight
	TableStyle.Options = table.Options{
		DoNotColorBordersAndSeparators: false,
		DrawBorder:                     false,
		SeparateColumns:                false,
		SeparateFooter:                 false,
		SeparateHeader:                 false,
		SeparateRows:                   false,
	}
}
