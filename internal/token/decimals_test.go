package token

import "testing"

func TestSmallestToFloatAndBack_Decimals9(t *testing.T) {
	const dec = 9
	oneUnitSmallest := uint64(1_000_000_000)

	f := SmallestToFloat(oneUnitSmallest, dec)
	if f != 1.0 {
		t.Fatalf("expected 1.0, got %f", f)
	}

	back := FloatToSmallest(f, dec)
	if back != oneUnitSmallest {
		t.Fatalf("round trip mismatch: want %d, got %d", oneUnitSmallest, back)
	}
}

func TestSmallestToFloatAndBack_Decimals6(t *testing.T) {
	const dec = 6
	amountSmallest := uint64(1_234_567) // 1.234567 units

	f := SmallestToFloat(amountSmallest, dec)
	if f <= 1.234566 || f >= 1.234568 {
		t.Fatalf("unexpected float value: %f", f)
	}

	back := FloatToSmallest(f, dec)
	if back != amountSmallest {
		t.Fatalf("round trip mismatch: want %d, got %d", amountSmallest, back)
	}
}

