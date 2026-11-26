#!/usr/bin/env python3
"""Keep additive cost snapshot types available in the older Free Fast overlays.

Only extend exact historical DTO declarations. Never copy candidate billing or
snapshot producers: the original Free Fast business RED remains unchanged.
"""
import pathlib
import subprocess
import sys

phase, baseline = sys.argv[1:]
if phase == 'usage':
    name = 'backend/internal/service/usage_log.go'
    marker = 'type UsageLog struct {'
    # Omit nil just as the pre-snapshot DTO did; persistence RED must reach
    # its original missing-snapshot assertion, not a later JSON null mismatch.
    addition = ('\n\tCustomerCostDetails *CustomerCostDetails `json:"CustomerCostDetails,omitempty"`\n'
                '\tProcurementCostSnapshot *ProcurementCostSnapshot `json:"ProcurementCostSnapshot,omitempty"`\n')
elif phase == 'admin-usage':
    name = 'backend/internal/handler/dto/types.go'
    marker = 'type AdminUsageLog struct {'
    addition = '\n\tProcurementCostSnapshot *service.ProcurementCostSnapshot `json:"procurement_cost_snapshot,omitempty"`\n'
elif phase == 'cache':
    name = 'backend/internal/service/customer_cost_details.go'
    marker = 'type CustomerCostDetails struct {'
    addition = ('\n\tProbePenalty *ProbePenaltyReceipt `json:"probe_penalty,omitempty"`\n'
                '\tFreeUltrafast bool `json:"free_ultrafast,omitempty"`\n'
                '\tCacheCostDetails []CustomerCacheCostDetail `json:"cache_cost_details,omitempty"`\n')
else:
    raise SystemExit('unsupported overlay')
path = pathlib.Path(name)
old = subprocess.check_output(['git', 'show', f'{baseline}:{name}'])
assert path.read_bytes() == old, f'expected exact historical declaration: {name}'
assert old.count(marker.encode()) == 1
new = old.replace(marker.encode(), (marker + addition).encode(), 1)
path.write_bytes(new)
print(f'cost snapshot DTO compatibility phase={phase} baseline={baseline} path={name}; producers and selected tests unchanged')
