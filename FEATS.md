trusted host, trusted time source ?

**Authenticity of the recording depends on the environment**: If the recorder runs on an untrusted host, adversary could have changed files before you recorded them or manipulated the recording process. Stronger attestation (TPM/secure enclave, remote timestamping, append-only logs) is needed for stronger claims.



**Protect signing keys and vouchers.**
 Keep private keys offline or in an HSM/TPM; store vouchers in append-only storage (WORM) or transparency log if you need non-repudiation.



**External timestamping for stronger claims.**
 A locally-signed voucher includes the recorder’s timestamp, but an adversary controlling the system clock can backdate. Use an RFC-3161 timestamping authority or public ledger (e.g., transparency log) if you need an authoritative time.



