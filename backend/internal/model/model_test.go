package model

import "testing"

func TestQuantity(t *testing.T) {
	for _, tt := range []struct {
		input, want string
		valid       bool
	}{{"0.100", "0.100", true}, {"0001.2", "1.200", true}, {"999999999999999.999", "999999999999999.999", true}, {"0", "0.000", true}, {"-1", "", false}, {"1.0001", "", false}, {"1000000000000000", "", false}, {"NaN", "", false}, {"1e3", "", false}, {" 1", "", false}, {"1,25", "", false}, {"", "", false}} {
		t.Run(tt.input, func(t *testing.T) {
			n, e := ParseQuantity(tt.input)
			if (e == nil) != tt.valid {
				t.Fatalf("valid=%v err=%v", tt.valid, e)
			}
			if e == nil && FormatQuantity(n) != tt.want {
				t.Fatalf("got %s", FormatQuantity(n))
			}
		})
	}
	a, _ := ParseQuantity("0.100")
	b, _ := ParseQuantity("0.200")
	if FormatQuantity(a+b) != "0.300" {
		t.Fatal("inexact sum")
	}
}
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

func FuzzQuantityRoundTrip(f *testing.F) {
	for _, seed := range []string{"0", "1.250", "999999999999999.999", "-1", "NaN", "0.0001"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		n, err := ParseQuantity(value)
		if err != nil {
			return
		}
		if n < 0 {
			t.Fatal("accepted negative quantity")
		}
		again, err := ParseQuantity(FormatQuantity(n))
		if err != nil || again != n {
			t.Fatalf("round trip changed %q", value)
		}
	})
}
