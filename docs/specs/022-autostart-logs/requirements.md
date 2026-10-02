# Requirements

- Auto-start must not leave an unbounded startup log beside the socket.
- Startup diagnostics and ongoing mutation logs must remain inspectable in a
  documented bounded state-directory location, including after the CLI exits.
- A verified `dibs daemon stop` removes its owned socket pid/legacy startup log.
  Unhealthy sockets, another database and manager-owned processes remain protected.
- Foreground and service-manager stderr logging stays unchanged.
- Regression tests cover routing, bounds/rotation and owned artifact cleanup;
  installed-binary verification uses isolated HOME/database/socket only.
