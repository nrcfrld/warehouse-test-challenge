package model

import "testing"

func TestItemValidation(t *testing.T) {
	in := NormalizeItem(ItemInput{SKU: " abc ", Name: " Widget ", Category: " Parts ", Unit: " kg "})
	if in.SKU != "ABC" || in.Name != "Widget" {
		t.Fatal(in)
	}
	if err := ValidateItem(in); err != nil {
		t.Fatal(err)
	}
	if err := ValidateItem(ItemInput{}); Public(err).Status != 400 {
		t.Fatalf("expected validation error: %v", err)
	}
}
