# AWS CloudWatch

Fluidify Regen receives CloudWatch alarms via Amazon SNS webhooks. When an alarm transitions to `ALARM` state, Regen creates an alert and optionally an incident.

## Webhook URL

```
POST https://your-domain.com/api/v1/webhooks/cloudwatch
```

## Setup

### Step 1: Create an SNS topic

```bash
aws sns create-topic --name fluidify-regen-alerts
```

Note the topic ARN: `arn:aws:sns:us-east-1:123456789:fluidify-regen-alerts`

### Step 2: Subscribe Regen as an HTTPS endpoint

```bash
aws sns subscribe \
  --topic-arn arn:aws:sns:us-east-1:123456789:fluidify-regen-alerts \
  --protocol https \
  --notification-endpoint https://your-domain.com/api/v1/webhooks/cloudwatch
```

Regen automatically confirms the subscription when it receives the confirmation request from SNS.

> **Note:** SNS requires HTTPS. Use a domain with a valid TLS certificate. For local testing, use an HTTPS tunnel (e.g. ngrok).

### Step 3: Add the SNS topic to a CloudWatch alarm

```bash
aws cloudwatch put-metric-alarm \
  --alarm-name "HighErrorRate" \
  --alarm-actions arn:aws:sns:us-east-1:123456789:fluidify-regen-alerts \
  --ok-actions arn:aws:sns:us-east-1:123456789:fluidify-regen-alerts \
  ...
```

Always add both `--alarm-actions` (firing) and `--ok-actions` (resolving) so Regen can close the alert automatically.

## How alarm fields map to Regen

| CloudWatch field | Regen field | Notes |
|---|---|---|
| `AlarmName` | Title | — |
| `AlarmDescription` | Description | — |
| `NewStateValue` | Status | `ALARM` → firing, `OK` → resolved |
| `AlarmArn` | ExternalID | Used for deduplication |
| `AWSAccountId` + region | Labels | Stored for context |
| `Trigger.MetricName` | Labels | Stored for context |

## Severity mapping

CloudWatch alarms don't have a native severity field. Regen maps based on alarm name keywords:

| Alarm name contains | Regen severity |
|---|---|
| `critical`, `p0`, `p1` | Critical |
| `warning`, `warn`, `p2` | Warning |
| *(anything else)* | Warning (default) |

To set severity explicitly, include it in the alarm name:
- `payments-api-error-rate-critical`
- `disk-usage-warning`
