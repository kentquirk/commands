// Code generated by "stringer -type=OrderStatus"; DO NOT EDIT.

package bitmart

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[Invalid-0]
	_ = x[Pending-1]
	_ = x[PartialSuccess-2]
	_ = x[Success-3]
	_ = x[Canceled-4]
	_ = x[PendingAndPartialSuccess-5]
	_ = x[SuccessAndCanceled-6]
}

const _OrderStatus_name = "InvalidPendingPartialSuccessSuccessCanceledPendingAndPartialSuccessSuccessAndCanceled"

var _OrderStatus_index = [...]uint8{0, 7, 14, 28, 35, 43, 67, 85}

func (i OrderStatus) String() string {
	if i < 0 || i >= OrderStatus(len(_OrderStatus_index)-1) {
		return "OrderStatus(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _OrderStatus_name[_OrderStatus_index[i]:_OrderStatus_index[i+1]]
}
