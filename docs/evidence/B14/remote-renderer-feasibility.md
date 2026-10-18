# B14 — Optional remote renderer feasibility
**Status:** BLOCKED before implementation; conditional scope not authorized by supplied prerequisites  
**Candidate SHA:** `77795c4301089f6ddd0b1bd5ab5f010ebf277c55`  
**Authority:** B14 plan, NFR-001/003/009/011, ADRs and source-policy constraints.

The planned renderer must run on a separate operator-owned host, use a bounded protocol and must not enable arbitrary URL access or bypass retailer terms. This workspace has no remote host, operator opt-in, approved retailer/source policy, credential-handling requirements or privacy protocol. No browser automation code/service was added and no renderer-related network traffic occurred. A feasibility decision cannot be completed beyond this explicit no-go until an operator supplies the separate target and authorizes an assessed protocol. Do not install a renderer on the Pi or exceed its whole-stack memory cap.
