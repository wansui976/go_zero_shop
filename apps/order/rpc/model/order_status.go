package model

// 状态转换表：当前状态 -> 允许转入的状态列表
var orderStatusTransitions = map[int64][]int64{
	0: {},        // Canceled 无法再转
	1: {2, 6, 7}, // Pending -> Paid, Completed (edge), Refunding (if refunded?), Refunding
	2: {3, 4, 7}, // Paid -> Delivering/Shipped/Refunding
	3: {4, 5},    // Delivering -> Shipped -> Received
	4: {5, 6},    // Shipped -> Received/Completed
	5: {6},       // Received -> Completed
	6: {},        // Completed terminal
	7: {8},       // Refunding -> Refunded
	8: {},        // Refunded terminal
}

// CanTransitionTo checks whether the order can change to newStatus
func (o *Orders) CanTransitionTo(newStatus int64) bool {
	allowed, ok := orderStatusTransitions[o.Status]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == newStatus {
			return true
		}
	}
	return false
}
