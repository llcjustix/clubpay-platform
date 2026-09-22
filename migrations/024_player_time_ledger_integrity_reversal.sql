-- Integrity corrections are recorded as immutable reversals rather than
-- mutating historical ledger rows. Keep the allowed kind list explicit so an
-- operator can audit these exceptional repairs separately from purchases and
-- ordinary session returns.
ALTER TABLE player_time_ledger
  DROP CONSTRAINT IF EXISTS player_time_ledger_kind_check;

ALTER TABLE player_time_ledger
  ADD CONSTRAINT player_time_ledger_kind_check CHECK (
    kind IN ('session_remaining', 'session_start', 'session_start_refund', 'manual_adjustment', 'balance_integrity_reversal')
  );
