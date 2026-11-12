# Run once from an Administrator PowerShell window if other devices cannot access the page.
New-NetFirewallRule -DisplayName 'Order Transaction LAN 18080' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 18080 -RemoteAddress LocalSubnet -Profile Any
