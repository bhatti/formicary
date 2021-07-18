package math

// RollingMinMax rolling min-max
type RollingMinMax struct {
	*RollingAverage
	Min int64
	Max int64
}

// NewRollingMinMax constructor
func NewRollingMinMax(capacity int) *RollingMinMax {
	maxUint := ^uint(0)
	return &RollingMinMax{
		RollingAverage: NewRollingAverage(capacity),
		Min:            int64(maxUint >> 1),
		Max:            0,
	}
}

// Add calculates min, max and average
func (r *RollingMinMax) Add(x int64) {
	r.Min = Min64(r.Min, x)
	r.Max = Max64(r.Max, x)
	r.RollingAverage.Add(x)
}
