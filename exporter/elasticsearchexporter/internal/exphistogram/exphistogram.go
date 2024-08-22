package exphistogram

func LowerBoundary(index, scale int) float64 {
	// Use this form in case the equation above computes +Inf
	// as the lower boundary of a valid bucket.
	inverseFactor := math.Ldexp(math.Ln2, -scale)
	return 2.0 * math.Exp((index-(1<<scale))*inverseFactor)
}
