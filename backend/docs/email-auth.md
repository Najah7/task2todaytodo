# Email authentication

## Signup, verification, and recovery

Signup creates an unverified account and sends a verification link. Opening the
link only renders a form; the user must submit the registration password to
confirm the address. Successful confirmation returns to login and does not
create a session.

Password recovery works for verified and unverified addresses. A submitted
reset form changes the password, verifies an unverified address, and invalidates
all personal access tokens and active verification/reset links. The user must
sign in with the new password. A reset is also the recovery path when someone
registered with a forgotten password or a third party pre-created an account.

Public signup, resend, and reset requests share a generic response regardless
of address existence or eligibility. One email-address keyed database limit
covers all three operations: one send attempt per minute and at most five in a
rolling hour. Attempts count even when delivery fails. Verification links last
one hour; reset links last 30 minutes. Only the newest link for its purpose is
valid. The database stores SHA-256 token digests, never raw link tokens.

Passwords are stored as Argon2id PHC values using a fresh random salt per
password (`m=19456 KiB,t=2,p=1`). Email delivery runs synchronously after the
database transaction commits, with a bounded send attempt. A delivery failure
leaves the account or link request recorded, returns the same generic public
response, and requires a manual resend after the cooldown; there is no mail
queue or automatic retry worker.

Mail is sent with both HTML and plain-text alternatives. Both put Japanese
first and English second; their two language buttons/links point to the same
action and token. The token stays in the URL fragment while a page is open,
which is omitted from page GET requests and referrers. It is sent only in the
explicit confirmation POST body. The frontend keeps the fragment while the
form is pending so a reload works,
then removes it when leaving after a successful action. Verification/reset
pages should be served with `Cache-Control: no-store` and
`Referrer-Policy: no-referrer`; do not load third-party assets on those pages.

## Local mail

The development Compose file starts Mailpit on loopback ports 1025 (SMTP) and
8025 (web UI). `make dev` wires the API container to Mailpit. When running the
API directly on the host with `air` or `go run`, keep `SMTP_HOST=localhost` in
`.env`; the Compose API container overrides this with `SMTP_HOST=mailpit`.
Open <http://localhost:8025> to inspect local messages. Mailpit is local-only;
do not expose its unauthenticated inbox on a public interface.

The isolated browser E2E Compose stack has its own PostgreSQL and Mailpit
instances and uses the same pinned Mailpit release. Its messages are polled by
the test runner; it never sends through SES.

## Production email

Production must set `ENVIRONMENT=production`, `EMAIL_PROVIDER=ses`,
`EMAIL_REGION`, `EMAIL_FROM_ADDRESS`, `EMAIL_FROM_NAME`, and an HTTPS
`FRONTEND_BASE_URL` origin with no path/query/fragment. The SES sender uses the
AWS SDK default credential chain; deploy with an instance/task/workload IAM
role that can call `ses:SendEmail`. Do not configure SMTP credentials for
production.

Before enabling delivery, configure and verify the sending domain or address in
SES in the selected region, publish the SES DKIM and recommended DMARC DNS
records, and request production access if the account remains in the SES
sandbox. SPF is needed for a custom MAIL FROM domain; SES's default MAIL FROM
domain already passes SPF. See the [SES custom MAIL FROM guide](https://docs.aws.amazon.com/ses/latest/dg/mail-from.html)
and [production access request guide](https://docs.aws.amazon.com/ses/latest/dg/request-production-access.html).
These are operator steps; the repository does not create cloud resources or
send setup mail.

## Future project invitations (design only)

Invitations are separate from signup, verification, and password reset. A
project invitation does not create a user or membership when sent. An
unregistered recipient follows the invite link, chooses a password, and accepts
in one transaction that creates a verified user, adds project membership, and
consumes the invitation. A verified existing recipient must first sign in
with the invited email and explicitly accept; another account/email cannot
accept it. An unverified existing recipient follows the invitation, chooses a
fresh password, and accepts in one transaction that reuses and verifies the
user, revokes prior sessions and auth links, adds membership, and consumes the
invitation. The invitation itself proves control of the address, so no extra
verification email is sent. Normal email verification never grants project
membership. This flow is design guidance only; there is no invitation endpoint,
table, or generic invitation engine yet.
