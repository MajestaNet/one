# Install-local product upgrade Automation (ADR-007).
# AWS admin confirms target images/version in Systems Manager → Automation.
# This is not the commercial Deploy API (customer metadata promote).
#
# Hardening:
# - Cluster/service/family/role targets are pinned (not caller-overridable).
# - Smoke-test credential is read from Secrets Manager (DEPLOY_SMOKE_API_KEY), never a plaintext param.
# - IAM scoped to this install's ECS resources.

variable "enable_upgrade_automation" {
  type        = bool
  default     = true
  description = "Create SSM Automation document for guided Majesta One product image rolls."
}

resource "aws_iam_role" "upgrade_automation" {
  count = var.enable_upgrade_automation ? 1 : 0
  name  = "${local.name}-upgrade-automation"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ssm.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy" "upgrade_automation" {
  count = var.enable_upgrade_automation ? 1 : 0
  name  = "${local.name}-upgrade-automation"
  role  = aws_iam_role.upgrade_automation[0].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "EcsRollMutate"
        Effect = "Allow"
        Action = [
          "ecs:UpdateService",
          "ecs:RegisterTaskDefinition",
        ]
        Resource = [
          "arn:aws:ecs:${var.aws_region}:${data.aws_caller_identity.current.account_id}:service/${local.name}/${local.name}-api",
          "arn:aws:ecs:${var.aws_region}:${data.aws_caller_identity.current.account_id}:service/${local.name}/${local.name}-worker",
          "arn:aws:ecs:${var.aws_region}:${data.aws_caller_identity.current.account_id}:task-definition/${local.name}-api:*",
          "arn:aws:ecs:${var.aws_region}:${data.aws_caller_identity.current.account_id}:task-definition/${local.name}-worker:*",
        ]
      },
      {
        Sid    = "EcsRollRead"
        Effect = "Allow"
        Action = [
          "ecs:DescribeServices",
          "ecs:DescribeTaskDefinition",
          "ecs:DescribeTasks",
          "ecs:ListTasks",
          "ecs:RegisterTaskDefinition",
        ]
        Resource = "*"
      },
      {
        Sid    = "PassRoles"
        Effect = "Allow"
        Action = ["iam:PassRole"]
        Resource = [
          aws_iam_role.ecs_execution.arn,
          aws_iam_role.ecs_api_task.arn,
          aws_iam_role.ecs_worker_task.arn,
        ]
      },
      {
        Sid      = "ReadSmokeSecret"
        Effect   = "Allow"
        Action   = ["secretsmanager:GetSecretValue"]
        Resource = [aws_secretsmanager_secret.install.arn]
      },
      {
        Sid      = "Logs"
        Effect   = "Allow"
        Action   = ["logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = "*"
      },
    ]
  })
}

locals {
  upgrade_automation_doc = {
    description   = "Confirm Majesta One product image upgrade: rolling ECS update, health + Deploy test gate, rollback on failure. Not for customer metadata promote (see /deploy/v1)."
    schemaVersion = "0.3"
    assumeRole    = var.enable_upgrade_automation ? aws_iam_role.upgrade_automation[0].arn : ""
    parameters = {
      ApiImage = {
        type        = "String"
        description = "Fully qualified API container image (repo:tag)"
      }
      WorkerImage = {
        type        = "String"
        description = "Fully qualified worker container image (repo:tag)"
      }
      ProductVersion = {
        type        = "String"
        description = "PRODUCT_VERSION string stamped on new task definitions"
      }
    }
    mainSteps = [
      {
        name   = "CapturePreviousRevisions"
        action = "aws:executeScript"
        inputs = {
          Runtime = "python3.11"
          Handler = "handler"
          InputPayload = {
            ClusterName       = aws_ecs_cluster.main.name
            ApiServiceName    = aws_ecs_service.api.name
            WorkerServiceName = aws_ecs_service.worker.name
          }
          Script = join("\n", [
            "def handler(events, context):",
            "    import boto3",
            "    ecs = boto3.client('ecs')",
            "    cluster = events['ClusterName']",
            "    out = {}",
            "    for key, svc in (('Api', events['ApiServiceName']), ('Worker', events['WorkerServiceName'])):",
            "        desc = ecs.describe_services(cluster=cluster, services=[svc])['services'][0]",
            "        out[f'{key}PreviousTaskDef'] = desc['taskDefinition']",
            "    return out",
          ])
        }
        outputs = [
          { Name = "ApiPreviousTaskDef", Selector = "$.Payload.ApiPreviousTaskDef", Type = "String" },
          { Name = "WorkerPreviousTaskDef", Selector = "$.Payload.WorkerPreviousTaskDef", Type = "String" },
        ]
      },
      {
        name   = "RegisterAndUpdateServices"
        action = "aws:executeScript"
        inputs = {
          Runtime = "python3.11"
          Handler = "handler"
          InputPayload = {
            ClusterName        = aws_ecs_cluster.main.name
            ApiServiceName     = aws_ecs_service.api.name
            WorkerServiceName  = aws_ecs_service.worker.name
            ApiTaskFamily      = aws_ecs_task_definition.api.family
            WorkerTaskFamily   = aws_ecs_task_definition.worker.family
            ApiImage           = "{{ ApiImage }}"
            WorkerImage        = "{{ WorkerImage }}"
            ProductVersion     = "{{ ProductVersion }}"
            ExecutionRoleArn   = aws_iam_role.ecs_execution.arn
            ApiTaskRoleArn     = aws_iam_role.ecs_api_task.arn
            WorkerTaskRoleArn  = aws_iam_role.ecs_worker_task.arn
          }
          Script = join("\n", [
            "def handler(events, context):",
            "    import copy, boto3",
            "    ecs = boto3.client('ecs')",
            "    def bump(family, image, product_version, exec_role, task_role):",
            "        cur = ecs.describe_task_definition(taskDefinition=family)['taskDefinition']",
            "        containers = copy.deepcopy(cur['containerDefinitions'])",
            "        for c in containers:",
            "            c['image'] = image",
            "            env = c.get('environment') or []",
            "            found = False",
            "            for e in env:",
            "                if e.get('name') == 'PRODUCT_VERSION':",
            "                    e['value'] = product_version",
            "                    found = True",
            "            if not found:",
            "                env.append({'name': 'PRODUCT_VERSION', 'value': product_version})",
            "            c['environment'] = env",
            "        reg = ecs.register_task_definition(",
            "            family=family,",
            "            taskRoleArn=task_role or cur.get('taskRoleArn'),",
            "            executionRoleArn=exec_role or cur.get('executionRoleArn'),",
            "            networkMode=cur.get('networkMode', 'awsvpc'),",
            "            containerDefinitions=containers,",
            "            requiresCompatibilities=cur.get('requiresCompatibilities') or ['FARGATE'],",
            "            cpu=cur.get('cpu'),",
            "            memory=cur.get('memory'),",
            "        )",
            "        return reg['taskDefinition']['taskDefinitionArn']",
            "    api_arn = bump(events['ApiTaskFamily'], events['ApiImage'], events['ProductVersion'], events['ExecutionRoleArn'], events['ApiTaskRoleArn'])",
            "    worker_arn = bump(events['WorkerTaskFamily'], events['WorkerImage'], events['ProductVersion'], events['ExecutionRoleArn'], events['WorkerTaskRoleArn'])",
            "    cluster = events['ClusterName']",
            "    ecs.update_service(cluster=cluster, service=events['ApiServiceName'], taskDefinition=api_arn, forceNewDeployment=True)",
            "    ecs.update_service(cluster=cluster, service=events['WorkerServiceName'], taskDefinition=worker_arn, forceNewDeployment=True)",
            "    return {'ApiTaskDef': api_arn, 'WorkerTaskDef': worker_arn}",
          ])
        }
        outputs = [
          { Name = "ApiTaskDef", Selector = "$.Payload.ApiTaskDef", Type = "String" },
          { Name = "WorkerTaskDef", Selector = "$.Payload.WorkerTaskDef", Type = "String" },
        ]
      },
      {
        name   = "WaitServicesStable"
        action = "aws:executeScript"
        onFailure = "step:RollbackServices"
        inputs = {
          Runtime = "python3.11"
          Handler = "handler"
          InputPayload = {
            ClusterName       = aws_ecs_cluster.main.name
            ApiServiceName    = aws_ecs_service.api.name
            WorkerServiceName = aws_ecs_service.worker.name
          }
          Script = join("\n", [
            "def handler(events, context):",
            "    import time, boto3",
            "    ecs = boto3.client('ecs')",
            "    cluster = events['ClusterName']",
            "    services = [events['ApiServiceName'], events['WorkerServiceName']]",
            "    deadline = time.time() + 900",
            "    while time.time() < deadline:",
            "        desc = ecs.describe_services(cluster=cluster, services=services)['services']",
            "        if all(s.get('deployments') and len(s['deployments']) == 1 and s['runningCount'] >= s['desiredCount'] for s in desc):",
            "            return {'stable': True}",
            "        time.sleep(15)",
            "    raise Exception('timed out waiting for ECS services to stabilize')",
          ])
        }
      },
      {
        name      = "HealthAndTests"
        action    = "aws:executeScript"
        onFailure = "step:RollbackServices"
        isEnd     = true
        inputs = {
          Runtime = "python3.11"
          Handler = "handler"
          InputPayload = {
            BaseUrl    = local.platform_public_url
            SecretArn  = aws_secretsmanager_secret.install.arn
          }
          Script = join("\n", [
            "def handler(events, context):",
            "    import json, urllib.request, urllib.error, boto3",
            "    base = events['BaseUrl'].rstrip('/')",
            "    sm = boto3.client('secretsmanager')",
            "    secret = json.loads(sm.get_secret_value(SecretId=events['SecretArn'])['SecretString'])",
            "    key = (secret.get('DEPLOY_SMOKE_API_KEY') or '').strip()",
            "    def get(path):",
            "        with urllib.request.urlopen(base + path, timeout=30) as resp:",
            "            if resp.status >= 400:",
            "                raise Exception(f'{path} returned {resp.status}')",
            "            return json.loads(resp.read().decode())",
            "    get('/healthz')",
            "    get('/readyz')",
            "    results = {'healthz': 'ok', 'readyz': 'ok', 'suites': []}",
            "    if not key:",
            "        results['note'] = 'DEPLOY_SMOKE_API_KEY empty in install secret; skipped PlatformSmoke/PostUpgradeSmoke'",
            "        return results",
            "    def run_suite(name, required=True):",
            "        req = urllib.request.Request(",
            "            base + '/deploy/v1/tests/runs',",
            "            data=json.dumps({'suiteApiName': name, 'trigger': 'product_upgrade'}).encode(),",
            "            headers={'authorization': f'Bearer {key}', 'content-type': 'application/json'},",
            "            method='POST',",
            "        )",
            "        try:",
            "            with urllib.request.urlopen(req, timeout=120) as resp:",
            "                body = json.loads(resp.read().decode())",
            "        except urllib.error.HTTPError as e:",
            "            body = e.read().decode() if hasattr(e, 'read') else str(e)",
            "            if e.code in (404, 400, 422) and not required:",
            "                return {'suite': name, 'skipped': True, 'reason': body[:500]}",
            "            raise Exception(f'suite {name} http {e.code}: {body[:2000]}')",
            "        run = body.get('run') or body",
            "        status = run.get('status')",
            "        if status != 'passed':",
            "            raise Exception(f'suite {name} status={status}: {json.dumps(body)[:2000]}')",
            "        return {'suite': name, 'status': status, 'id': run.get('id')}",
            "    results['suites'].append(run_suite('PlatformSmoke', required=True))",
            "    results['suites'].append(run_suite('PostUpgradeSmoke', required=False))",
            "    return results",
          ])
        }
      },
      {
        name   = "RollbackServices"
        action = "aws:executeScript"
        isEnd  = true
        inputs = {
          Runtime = "python3.11"
          Handler = "handler"
          InputPayload = {
            ClusterName           = aws_ecs_cluster.main.name
            ApiServiceName        = aws_ecs_service.api.name
            WorkerServiceName     = aws_ecs_service.worker.name
            ApiPreviousTaskDef    = "{{ CapturePreviousRevisions.ApiPreviousTaskDef }}"
            WorkerPreviousTaskDef = "{{ CapturePreviousRevisions.WorkerPreviousTaskDef }}"
          }
          Script = join("\n", [
            "def handler(events, context):",
            "    import boto3",
            "    ecs = boto3.client('ecs')",
            "    cluster = events['ClusterName']",
            "    ecs.update_service(cluster=cluster, service=events['ApiServiceName'], taskDefinition=events['ApiPreviousTaskDef'], forceNewDeployment=True)",
            "    ecs.update_service(cluster=cluster, service=events['WorkerServiceName'], taskDefinition=events['WorkerPreviousTaskDef'], forceNewDeployment=True)",
            "    return {'rolledBack': True, 'api': events['ApiPreviousTaskDef'], 'worker': events['WorkerPreviousTaskDef']}",
          ])
        }
      },
    ]
  }
}

resource "aws_ssm_document" "product_upgrade" {
  count           = var.enable_upgrade_automation ? 1 : 0
  name            = "One-ProductUpgrade-${replace(local.name, "_", "-")}"
  document_type   = "Automation"
  document_format = "JSON"
  content         = jsonencode(local.upgrade_automation_doc)

  tags = {
    Name = "${local.name}-product-upgrade"
  }
}
