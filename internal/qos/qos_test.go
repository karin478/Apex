package qos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 1. TestPriorityValue
// ---------------------------------------------------------------------------

func TestPriorityValue(t *testing.T) {
	assert.Equal(t, 0, PriorityValue("URGENT"))
	assert.Equal(t, 1, PriorityValue("HIGH"))
	assert.Equal(t, 2, PriorityValue("NORMAL"))
	assert.Equal(t, 3, PriorityValue("LOW"))
	assert.Equal(t, 99, PriorityValue("WHATEVER"))
	assert.Equal(t, 99, PriorityValue(""))
}

// ---------------------------------------------------------------------------
// 2. TestNewSlotPool
// ---------------------------------------------------------------------------

func TestNewSlotPool(t *testing.T) {
	pool := NewSlotPool(8)
	require.NotNil(t, pool)
	assert.Equal(t, 8, pool.Total())
	assert.Equal(t, 8, pool.Available())
	assert.Equal(t, 0, pool.Usage().Used)
}

// ---------------------------------------------------------------------------
// 3. TestAddReservation
// ---------------------------------------------------------------------------

func TestAddReservation(t *testing.T) {
	pool := NewSlotPool(8)

	// Adding URGENT(2) should succeed.
	err := pool.AddReservation(Reservation{Priority: "URGENT", Reserved: 2})
	require.NoError(t, err)

	// Adding HIGH(2) should succeed (total reserved = 4 <= 8).
	err = pool.AddReservation(Reservation{Priority: "HIGH", Reserved: 2})
	require.NoError(t, err)

	// Adding LOW(5) should fail (total reserved = 2+2+5 = 9 > 8).
	err = pool.AddReservation(Reservation{Priority: "LOW", Reserved: 5})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds pool capacity")

	// Verify only URGENT and HIGH reservations exist, sorted by priority.
	reservations := pool.Reservations()
	require.Len(t, reservations, 2)
	assert.Equal(t, "URGENT", reservations[0].Priority)
	assert.Equal(t, "HIGH", reservations[1].Priority)
}

// ---------------------------------------------------------------------------
// 4. TestAllocateRelease
// ---------------------------------------------------------------------------

func TestAllocateRelease(t *testing.T) {
	pool := NewSlotPool(8)

	// Allocate 3 NORMAL slots.
	for i := 0; i < 3; i++ {
		ok := pool.Allocate("NORMAL")
		require.True(t, ok, "allocation %d should succeed", i+1)
	}
	assert.Equal(t, 3, pool.Usage().Used)
	assert.Equal(t, 5, pool.Available())

	// Release 1 NORMAL slot.
	pool.Release("NORMAL")
	assert.Equal(t, 2, pool.Usage().Used)
	assert.Equal(t, 6, pool.Available())

	// Release for a priority with 0 allocations is a no-op.
	pool.Release("HIGH")
	assert.Equal(t, 2, pool.Usage().Used)
}

// ---------------------------------------------------------------------------
// 5. TestAllocateRespectReservation
// ---------------------------------------------------------------------------

func TestAllocateRespectReservation(t *testing.T) {
	// Pool=4, URGENT reserved=2 => unreserved=2.
	pool := NewSlotPool(4)
	require.NoError(t, pool.AddReservation(Reservation{Priority: "URGENT", Reserved: 2}))

	// Allocate 2 LOW -- should consume the 2 unreserved slots.
	assert.True(t, pool.Allocate("LOW"))
	assert.True(t, pool.Allocate("LOW"))

	// 3rd LOW should fail -- unreserved is full and LOW cannot borrow from
	// URGENT's reserved slots (lower priority cannot borrow from higher).
	assert.False(t, pool.Allocate("LOW"))

	// But URGENT can still allocate (it has 2 reserved slots).
	assert.True(t, pool.Allocate("URGENT"))
	assert.True(t, pool.Allocate("URGENT"))

	// Pool is now full.
	assert.Equal(t, 0, pool.Available())
	assert.False(t, pool.Allocate("URGENT"))
}

// ---------------------------------------------------------------------------
// 6. TestAllocateUrgentAlways
// ---------------------------------------------------------------------------

func TestAllocateUrgentAlways(t *testing.T) {
	// Pool=4, URGENT reserved=2, LOW reserved=2.
	pool := NewSlotPool(4)
	require.NoError(t, pool.AddReservation(Reservation{Priority: "URGENT", Reserved: 2}))
	require.NoError(t, pool.AddReservation(Reservation{Priority: "LOW", Reserved: 2}))

	// Allocate 2 LOW (uses LOW's reserved slots).
	assert.True(t, pool.Allocate("LOW"))
	assert.True(t, pool.Allocate("LOW"))

	// URGENT should still be able to allocate using its own reserved slots.
	assert.True(t, pool.Allocate("URGENT"))
	assert.True(t, pool.Allocate("URGENT"))

	// Pool is now full.
	assert.Equal(t, 0, pool.Available())
	assert.Equal(t, 4, pool.Usage().Used)
}

// ---------------------------------------------------------------------------
// 7. TestAllocateBorrowFromLowerPriority (Step 4 direct coverage)
// ---------------------------------------------------------------------------

func TestAllocateBorrowFromLowerPriority(t *testing.T) {
	// Pool=4, HIGH reserved=2 → unreserved=2
	pool := NewSlotPool(4)
	require.NoError(t, pool.AddReservation(Reservation{Priority: "HIGH", Reserved: 2}))

	// Fill the unreserved pool with NORMAL.
	assert.True(t, pool.Allocate("NORMAL"))
	assert.True(t, pool.Allocate("NORMAL"))

	// NORMAL (priority=2) cannot borrow from HIGH (priority=1) reserved slots.
	assert.False(t, pool.Allocate("NORMAL"))

	// URGENT (priority=0) can borrow from HIGH's unused reserved (Step 4).
	assert.True(t, pool.Allocate("URGENT"))
	assert.True(t, pool.Allocate("URGENT"))

	// Pool is fully utilized now (4/4).
	assert.Equal(t, 0, pool.Available())
}

// ---------------------------------------------------------------------------
// 8. TestBorrowLimitRespected (regression: borrow must not exceed donor capacity)
// ---------------------------------------------------------------------------

func TestBorrowLimitRespected(t *testing.T) {
	// Pool=6, HIGH reserved=2, LOW reserved=2 → unreserved=2
	pool := NewSlotPool(6)
	require.NoError(t, pool.AddReservation(Reservation{Priority: "HIGH", Reserved: 2}))
	require.NoError(t, pool.AddReservation(Reservation{Priority: "LOW", Reserved: 2}))

	// NORMAL fills unreserved (2 slots).
	assert.True(t, pool.Allocate("NORMAL"))
	assert.True(t, pool.Allocate("NORMAL"))

	// NORMAL can borrow from LOW's 2 reserved slots (priority 2 < 3).
	assert.True(t, pool.Allocate("NORMAL"))
	assert.True(t, pool.Allocate("NORMAL"))

	// NORMAL cannot borrow more — LOW's 2 reserved slots are exhausted.
	assert.False(t, pool.Allocate("NORMAL"), "NORMAL should not steal HIGH's reserved slots")

	// HIGH can still use its own reserved slots.
	assert.True(t, pool.Allocate("HIGH"), "HIGH should still access its reserved slots")
	assert.True(t, pool.Allocate("HIGH"), "HIGH should still access its reserved slots")

	assert.Equal(t, 0, pool.Available())
}

// ---------------------------------------------------------------------------
// 9. TestUsage
// ---------------------------------------------------------------------------

func TestUsage(t *testing.T) {
	pool := NewSlotPool(10)
	require.NoError(t, pool.AddReservation(Reservation{Priority: "HIGH", Reserved: 3}))

	// Allocate some slots.
	pool.Allocate("HIGH")
	pool.Allocate("HIGH")
	pool.Allocate("NORMAL")

	usage := pool.Usage()
	assert.Equal(t, 10, usage.Total)
	assert.Equal(t, 3, usage.Used)
	assert.Equal(t, 7, usage.Available)
	assert.Equal(t, 2, usage.ByPriority["HIGH"])
	assert.Equal(t, 1, usage.ByPriority["NORMAL"])

	// Verify ByPriority is a copy -- mutating it must not affect the pool.
	usage.ByPriority["HIGH"] = 999
	assert.Equal(t, 2, pool.Usage().ByPriority["HIGH"])
}
