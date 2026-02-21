package main

import (
	"fmt"

	"github.com/lyndonlyu/apex/internal/qos"
	"github.com/spf13/cobra"
)

var qosFormat string

var qosCmd = &cobra.Command{
	Use:   "qos",
	Short: "Resource quality of service",
}

var qosStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show slot pool usage",
	RunE:  runQoSStatus,
}

var qosReservationsCmd = &cobra.Command{
	Use:   "reservations",
	Short: "List slot reservations",
	RunE:  runQoSReservations,
}

func init() {
	qosStatusCmd.Flags().StringVar(&qosFormat, "format", "", "Output format (json)")
	qosCmd.AddCommand(qosStatusCmd, qosReservationsCmd)
}

func defaultQoSPool() *qos.SlotPool {
	pool := qos.NewSlotPool(8)
	pool.AddReservation(qos.Reservation{Priority: "URGENT", Reserved: 2})
	pool.AddReservation(qos.Reservation{Priority: "HIGH", Reserved: 2})
	return pool
}

func runQoSStatus(cmd *cobra.Command, args []string) error {
	pool := defaultQoSPool()
	usage := pool.Usage()

	if qosFormat == "json" {
		out, err := qos.FormatUsageJSON(usage)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(qos.FormatUsage(usage))
	}
	return nil
}

func runQoSReservations(cmd *cobra.Command, args []string) error {
	pool := defaultQoSPool()
	fmt.Print(qos.FormatReservations(pool.Reservations()))
	return nil
}
