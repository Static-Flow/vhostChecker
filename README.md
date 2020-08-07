# vhostChecker
VHostChecker takes a csv list of targets in the form of domain,ip,port and runs 3 seperate connection checks to get a quick sense of how the target handles Host Header changes and another check using the domain to see if the target is only accessible via IP.

## Checks Performed

1) Fetch ```https://<IP>:<PORT> Host: <IP>```
2) Fetch ```https://<IP>:<PORT> Host: <DOMAIN>```
3) Fetch ```https://<IP>:<PORT> Host: LOCALHOST```
4) Fetch ```https://<DOMAIN>:<PORT> Host: <DOMAIN>```
