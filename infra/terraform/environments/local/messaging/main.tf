# Messaging do fan-out durável (ADR-0020): tópico SNS match-results + uma fila
# SQS por sink (achievements/leaderboard) com DLQ e redrive. Ambiente LOCAL:
# aponta para o LocalStack (ADR-0004), mesmo padrão do MinIO/S3. O mesmo módulo
# roda contra a AWS real trocando os endpoints/credenciais (estreia do Terraform
# no projeto — ver plan da spec 006).

terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# Provider apontando o LocalStack. Credenciais fake; validações de conta/metadata
# desligadas (não há IAM real no emulador).
provider "aws" {
  region                      = "us-east-1"
  access_key                  = "test"
  secret_key                  = "test"
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true

  endpoints {
    sns = "http://localstack:4566"
    sqs = "http://localstack:4566"
  }
}

# maxReceiveCount: tentativas antes de a mensagem ir para a DLQ.
variable "max_receive_count" {
  type    = number
  default = 5
}

resource "aws_sns_topic" "match_results" {
  name = "match-results"
}

# ── achievements ───────────────────────────────────────────────────────────
resource "aws_sqs_queue" "achievements_dlq" {
  name = "match-results-achievements-dlq"
}

resource "aws_sqs_queue" "achievements" {
  name                       = "match-results-achievements"
  visibility_timeout_seconds = 30
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.achievements_dlq.arn
    maxReceiveCount     = var.max_receive_count
  })
}

# ── leaderboard ────────────────────────────────────────────────────────────
resource "aws_sqs_queue" "leaderboard_dlq" {
  name = "match-results-leaderboard-dlq"
}

resource "aws_sqs_queue" "leaderboard" {
  name                       = "match-results-leaderboard"
  visibility_timeout_seconds = 30
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.leaderboard_dlq.arn
    maxReceiveCount     = var.max_receive_count
  })
}

# ── analytics ──────────────────────────────────────────────────────────────
resource "aws_sqs_queue" "analytics_dlq" {
  name = "match-results-analytics-dlq"
}

resource "aws_sqs_queue" "analytics" {
  name                       = "match-results-analytics"
  visibility_timeout_seconds = 30
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.analytics_dlq.arn
    maxReceiveCount     = var.max_receive_count
  })
}

# ── assinaturas SNS→SQS (raw delivery: o corpo da SQS = o payload publicado,
#    sem o envelope de Notification do SNS — o consumidor lê o JSON direto) ───
resource "aws_sns_topic_subscription" "achievements" {
  topic_arn            = aws_sns_topic.match_results.arn
  protocol             = "sqs"
  endpoint             = aws_sqs_queue.achievements.arn
  raw_message_delivery = true
}

resource "aws_sns_topic_subscription" "leaderboard" {
  topic_arn            = aws_sns_topic.match_results.arn
  protocol             = "sqs"
  endpoint             = aws_sqs_queue.leaderboard.arn
  raw_message_delivery = true
}

resource "aws_sns_topic_subscription" "analytics" {
  topic_arn            = aws_sns_topic.match_results.arn
  protocol             = "sqs"
  endpoint             = aws_sqs_queue.analytics.arn
  raw_message_delivery = true
}

# Permite o SNS entregar nas filas (parity com AWS; o LocalStack não força).
data "aws_iam_policy_document" "allow_sns" {
  for_each = {
    achievements = aws_sqs_queue.achievements.arn
    leaderboard  = aws_sqs_queue.leaderboard.arn
    analytics    = aws_sqs_queue.analytics.arn
  }
  statement {
    effect    = "Allow"
    actions   = ["sqs:SendMessage"]
    resources = [each.value]
    principals {
      type        = "Service"
      identifiers = ["sns.amazonaws.com"]
    }
    condition {
      test     = "ArnEquals"
      variable = "aws:SourceArn"
      values   = [aws_sns_topic.match_results.arn]
    }
  }
}

resource "aws_sqs_queue_policy" "achievements" {
  queue_url = aws_sqs_queue.achievements.id
  policy    = data.aws_iam_policy_document.allow_sns["achievements"].json
}

resource "aws_sqs_queue_policy" "leaderboard" {
  queue_url = aws_sqs_queue.leaderboard.id
  policy    = data.aws_iam_policy_document.allow_sns["leaderboard"].json
}

resource "aws_sqs_queue_policy" "analytics" {
  queue_url = aws_sqs_queue.analytics.id
  policy    = data.aws_iam_policy_document.allow_sns["analytics"].json
}

output "topic_arn" {
  value = aws_sns_topic.match_results.arn
}

output "achievements_queue_url" {
  value = aws_sqs_queue.achievements.id
}

output "leaderboard_queue_url" {
  value = aws_sqs_queue.leaderboard.id
}

output "analytics_queue_url" {
  value = aws_sqs_queue.analytics.id
}
