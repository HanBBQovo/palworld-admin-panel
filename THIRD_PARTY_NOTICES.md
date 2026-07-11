# Third-party notices

## Palworld map asset and coordinate mapping

- Project: RNZ01/palworld-server-dashboard
- Source: https://github.com/RNZ01/palworld-server-dashboard
- License: MIT
- Files used or adapted:
  - `public/palworld-map/full-map-z4.png`
  - Coordinate-to-screen mapping from `components/live-map.tsx`

## World save index integration

- Project: oMaN-Rod/palworld-save-pal and oMaN-Rod/palworld-save-tools
- Sources: https://github.com/oMaN-Rod/palworld-save-pal and https://github.com/oMaN-Rod/palworld-save-tools
- Versions: Palworld Save Pal v0.17.4; palworld-save-tools commit a78c3c13058abb534aa31a440d57cba17b9e3210
- License: palworld-save-tools is MIT; Palworld Save Pal does not publish a root license file as of v0.17.4.
- Integration: the project-owned read-only HTTP adapter loads the separately downloaded Save Pal release at image build time. No upstream source or binary is committed here.

## Maintenance save editor integration

- Project: oMaN-Rod/palworld-save-pal
- Source: https://github.com/oMaN-Rod/palworld-save-pal
- Version: v0.17.4
- License: the upstream repository does not publish a root license file as of v0.17.4.
- Integration: optional maintenance-only external release; no source code or binary is committed to this project.
