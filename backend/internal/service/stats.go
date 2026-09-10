package service

// Statistic counter names kept in the per-session Redis hash. The dashboard
// reads them via GET /api/v1/simulations/{id}/stats and the final values are
// snapshotted into simulation_sessions.stats_snapshot when a session stops.
const (
	StatTransmitTotal        = "transmit_total"
	StatDroppedInvalid       = "dropped_invalid"
	StatDroppedSFLimit       = "dropped_sf_limit"
	StatDroppedMaxHops       = "dropped_max_hops"
	StatDroppedAirLoss       = "dropped_air_loss"
	StatDroppedOutOfRange    = "dropped_out_of_range"
	StatDroppedNoPosition    = "dropped_no_position"
	StatDroppedTargetOffline = "dropped_target_offline"
	StatDuplicatesFiltered   = "duplicates_filtered"
	StatForwardedToNode      = "forwarded_to_node"
	StatDeliveredToEdge      = "delivered_to_edge"
	StatAcksRelayed          = "acks_relayed"
	StatAcksDropped          = "acks_dropped"
	StatAcksStale            = "acks_stale"
	StatPingsAccepted        = "pings_accepted"
	StatPingsRateLimited     = "pings_rate_limited"
)
