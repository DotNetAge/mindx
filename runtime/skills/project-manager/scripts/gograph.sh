#!/bin/bash
#
# gograph.sh - MindX Project Manager graph database operations script
# Wraps the gograph CLI to provide CRUD operations for project management
#

set -e

# ====== Configuration ======
GOGRAPH_CMD="${GOGRAPH_CMD:-gograph}"
DB_PATH="${PROJECT_DB_PATH:-runtime/data/project-graph.db}"

# ====== Utility Functions ======

usage() {
    cat << EOF
MindX Project Manager - gograph Operations Script

Usage: $0 <command> [options]

Commands:
  Project Management:
    create-project   Create a project node
    query-project    Query project details
    list-projects    List all projects
    update-project   Update project properties

  Goal Management:
    create-goal      Create a goal node (under a project)
    query-goals      Query all goals for a project
    update-goal      Update goal progress and status

  Task Management:
    create-task      Create a task node (under a goal)
    update-task      Update task status and results
    record-execution Record a task execution result
    query-tasks      Query tasks by goal or status filter
    query-by-status  Query tasks by single status
    get-task         Get a single task with full details
    get-task-output  Get task output for quality verification

  Session Management (SubAgent lifecycle tracking):
    register-session Register a new session for a task
    get-session      Get session details including interruption context
    update-session   Update session state after recovery action
    query-sessions   Find stale/interrupted sessions needing attention

  Relationship Management:
    add-dependency   Add a task dependency
    remove-dependency Remove a dependency

  Query Tools:
    get-goal         Get a single goal with full details
    progress-report  Generate a progress report dataset

Global Options:
    --db-path PATH   Database path (default: ${DB_PATH})
    --help           Show this help message

Examples:
  $0 create-project --name "Community Operations" --description "Increase engagement"
  $0 create-goal --project-id proj001 --title "Content Creation" --weight 0.4
  $0 create-task --goal-id goal001 --agent @writer --cron "0 0 9 * * 1" --prompt "Write article"
  $0 update-task --task-id task001 --status completed --scheduler-id sched123 --session-id sess456
  $0 register-session --task-id task001 --agent @writer --session-status initialized
  $0 get-session --session-id sess456
  $0 update-session --session-id sess456 --status resumed --resolution user_authorized
  $0 query-sessions --status "awaiting_*" --stale-threshold "24h"
  $0 query-tasks --status "in_progress,awaiting_authorization,awaiting_clarification"
  $0 get-task-output --task-id task001
  $0 progress-report --project-id proj001
EOF
}

check_gograph() {
    if ! command -v "$GOGRAPH_CMD" &> /dev/null; then
        echo "❌ Error: gograph command not found"
        echo "   Ensure gograph is installed and in PATH"
        exit 1
    fi
}

run_cypher() {
    local cypher="$1"
    "$GOGRAPH_CMD" -d "$DB_PATH" -c "$cypher" 2>/dev/null
}

generate_uuid() {
    if command -v uuidgen &> /dev/null; then
        uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8
    else
        cat /proc/sys/kernel/random/uuid 2>/dev/null | cut -c1-8 || date +%s | md5sum | cut -c1-8
    fi
}

get_timestamp() {
    date -u +%Y-%m-%dT%H:%M:%SZ
}

json_escape() {
    echo "$1" | sed 's/"/\\"/g' | sed "s/'/\\\\'/g"
}

# ====== Project Management Commands ======

cmd_create_project() {
    local name="" description="" metrics="{}" timeline="{}"

    while [[ $# -gt 0 ]]; do
        case $1 in
            --name) name="$2"; shift 2 ;;
            --description) description="$2"; shift 2 ;;
            --metrics) metrics="$2"; shift 2 ;;
            --timeline) timeline="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$name" ]]; then
        echo "❌ Missing required parameter: --name"
        exit 1
    fi

    local id="proj-$(generate_uuid)"
    local now=$(get_timestamp)

    local cypher="
        CREATE (p:Project {
            id: '$id',
            name: '$(json_escape "$name")',
            description: '$(json_escape "${description:-$name}")',
            status: 'active',
            progress: 0.0,
            created_at: '$now',
            updated_at: '$now',
            metrics: $metrics,
            timeline: $timeline
        })
        RETURN p.id as id, p.name as name, p.status as status
    "

    local result=$(run_cypher "$cypher")
    echo "✅ Project created:"
    echo "   ID: $id"
    echo "   Name: $name"
    echo "   Status: active"
}

cmd_query_project() {
    local project_id=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --project-id) project_id="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$project_id" ]]; then
        echo "❌ Missing required parameter: --project-id"
        exit 1
    fi

    local cypher="
        MATCH (p:Project {id: '$project_id'})
        OPTIONAL MATCH (p)-[:HAS_GOAL]->(g:Goal)
        OPTIONAL MATCH (g)-[:CONTAINS]->(t:Task)
        RETURN p,
               collect(DISTINCT g) as goals,
               collect(DISTINCT t) as tasks
    "

    run_cypher "$cypher"
}

cmd_list_projects() {
    local cypher="
        MATCH (p:Project)
        OPTIONAL MATCH (p)-[:HAS_GOAL]->(g:Goal)
        RETURN p.id as id,
               p.name as name,
               p.status as status,
               p.progress as progress,
               count(g) as goal_count
        ORDER BY p.updated_at DESC
    "

    run_cypher "$cypher"
}

cmd_update_project() {
    local project_id="" status="" progress=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --project-id) project_id="$2"; shift 2 ;;
            --status) status="$2"; shift 2 ;;
            --progress) progress="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$project_id" ]]; then
        echo "❌ Missing required parameter: --project-id"
        exit 1
    fi

    local updates=""
    local now=$(get_timestamp)

    [[ -n "$status" ]] && updates+=", status: '$status'"
    [[ -n "$progress" ]] && updates+=", progress: $progress"
    updates+=" , updated_at: '$now'"

    local cypher="
        MATCH (p:Project {id: '$project_id'})
        SET ${updates#,}
        RETURN p.id, p.name, p.status, p.progress
    "

    run_cypher "$cypher"
}

# ====== Goal Management Commands ======

cmd_create_goal() {
    local project_id="" title="" description="" weight="" metrics="{}"

    while [[ $# -gt 0 ]]; do
        case $1 in
            --project-id) project_id="$2"; shift 2 ;;
            --title) title="$2"; shift 2 ;;
            --description) description="$2"; shift 2 ;;
            --weight) weight="$2"; shift 2 ;;
            --metrics) metrics="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$project_id" || -z "$title" ]]; then
        echo "❌ Missing required parameters: --project-id, --title"
        exit 1
    fi

    local id="goal-$(generate_uuid)"
    local now=$(get_timestamp)

    local cypher="
        MATCH (p:Project {id: '$project_id'})
        CREATE (g:Goal {
            id: '$id',
            title: '$(json_escape "$title")',
            description: '$(json_escape "${description:-$title}")',
            weight: ${weight:-0.0},
            status: 'pending',
            progress: 0.0,
            created_at: '$now',
            updated_at: '$now',
            metrics: $metrics
        })
        CREATE (p)-[:HAS_GOAL {order: timestamp()}]->(g)
        RETURN g.id as id, g.title as title, g.weight as weight
    "

    run_cypher "$cypher"
}

cmd_query_goals() {
    local project_id=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --project-id) project_id="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$project_id" ]]; then
        echo "❌ Missing required parameter: --project-id"
        exit 1
    fi

    local cypher="
        MATCH (p:Project {id: '$project_id'})-[:HAS_GOAL]->(g:Goal)
        OPTIONAL MATCH (g)-[:CONTAINS]->(t:Task)
        RETURN g.id, g.title, g.status, g.progress, g.weight,
               count(t) as task_count,
               count(CASE WHEN t.status = 'completed' THEN 1 END) as completed_count
        ORDER BY g.created_at
    "

    run_cypher "$cypher"
}

cmd_update_goal() {
    local goal_id="" status="" progress=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --goal-id) goal_id="$2"; shift 2 ;;
            --status) status="$2"; shift 2 ;;
            --progress) progress="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$goal_id" ]]; then
        echo "❌ Missing required parameter: --goal-id"
        exit 1
    fi

    local updates=""
    local now=$(get_timestamp)

    [[ -n "$status" ]] && updates+=", status: '$status'"
    [[ -n "$progress" ]] && updates+=", progress: $progress"
    updates+=" , updated_at: '$now'"

    # If status becomes completed, set progress to 100%
    if [[ "$status" == "completed" && -z "$progress" ]]; then
        updates+=", progress: 1.0"
    fi

    local cypher="
        MATCH (g:Goal {id: '$goal_id'})
        SET ${updates#,}
        RETURN g.id, g.title, g.status, g.progress
    "

    run_cypher "$cypher"
}

# ====== Task Management Commands ======

cmd_create_task() {
    local goal_id="" title="" agent="" cron_expr="" prompt="" priority="normal"

    while [[ $# -gt 0 ]]; do
        case $1 in
            --goal-id) goal_id="$2"; shift 2 ;;
            --title) title="$2"; shift 2 ;;
            --agent) agent="$2"; shift 2 ;;
            --cron-expr) cron_expr="$2"; shift 2 ;;
            --prompt) prompt="$2"; shift 2 ;;
            --priority) priority="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$goal_id" || -z "$title" ]]; then
        echo "❌ Missing required parameters: --goal-id, --title"
        exit 1
    fi

    local id="task-$(generate_uuid)"
    local now=$(get_timestamp)

    local cypher="
        MATCH (g:Goal {id: '$goal_id'})
        CREATE (t:Task {
            id: '$id',
            title: '$(json_escape "$title")',
            agent: '${agent:-@assistant}',
            cron_expr: '$(json_escape "${cron_expr:-}")',
            prompt: '$(json_escape "${prompt:-$title}")',
            status: 'pending',
            priority: '$priority',
            progress: 0.0,
            success_count: 0,
            failure_count: 0,
            created_at: '$now',
            updated_at: '$now'
        })
        CREATE (g)-[:CONTAINS {order: timestamp()}]->(t)
        RETURN t.id as id, t.title as title, t.agent as agent, t.status as status
    "

    run_cypher "$cypher"
}

cmd_update_task() {
    local task_id="" status="" result="" scheduler_id="" progress=""
    local session_id="" interruption_type="" interruption_context="" verification_note=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --task-id) task_id="$2"; shift 2 ;;
            --status) status="$2"; shift 2 ;;
            --result) result="$2"; shift 2 ;;
            --scheduler-id) scheduler_id="$2"; shift 2 ;;
            --progress) progress="$2"; shift 2 ;;
            --session-id) session_id="$2"; shift 2 ;;
            --interruption-type) interruption_type="$2"; shift 2 ;;
            --interruption-context) interruption_context="$2"; shift 2 ;;
            --verification-note) verification_note="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$task_id" ]]; then
        echo "❌ Missing required parameter: --task-id"
        exit 1
    fi

    local updates=""
    local now=$(get_timestamp)

    [[ -n "$status" ]] && updates+=", status: '$status'"
    [[ -n "$result" ]] && updates+=", summary: '$(json_escape "$result")'"
    [[ -n "$scheduler_id" ]] && updates+=", scheduler_id: '$scheduler_id'"
    [[ -n "$progress" ]] && updates+=", progress: $progress"
    [[ -n "$session_id" ]] && updates+=", session_id: '$session_id'"
    [[ -n "$interruption_type" ]] && updates+=", interruption_type: '$interruption_type'"
    [[ -n "$interruption_context" ]] && updates+=", interruption_context: $interruption_context"
    [[ -n "$verification_note" ]] && updates+=", verification_note: '$(json_escape "$verification_note")'"

    if [[ "$status" == "completed" ]]; then
        updates+=", success_count: coalesce(success_count, 0) + 1"
        updates+=", progress: 1.0"
    elif [[ "$status" == "failed" ]]; then
        updates+=", failure_count: coalesce(failure_count, 0) + 1"
    fi

    updates+=" , updated_at: '$now'"

    local cypher="
        MATCH (t:Task {id: '$task_id'})
        SET ${updates#,}
        RETURN t.id, t.title, t.status, t.progress, t.success_count, t.failure_count, t.session_id
    "

    run_cypher "$cypher"
}

cmd_record_execution() {
    local task_id="" status="" result="" duration=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --task-id) task_id="$2"; shift 2 ;;
            --status) status="$2"; shift 2 ;;
            --result) result="$2"; shift 2 ;;
            --duration) duration="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$task_id" || -z "$status" ]]; then
        echo "❌ Missing required parameters: --task-id, --status"
        exit 1
    fi

    local exec_id="exec-$(generate_uuid)"
    local now=$(get_timestamp)

    # Create execution record node
    local cypher_exec="
        MATCH (t:Task {id: '$task_id'})
        CREATE (e:Execution {
            id: '$exec_id',
            status: '$status',
            result: '$(json_escape "${result:-}")',
            duration_seconds: ${duration:-0},
            executed_at: '$now'
        })
        CREATE (t)-[:HAS_EXECUTION]->(e)
    "

    run_cypher "$cypher_exec" > /dev/null

    # Update task status
    cmd_update_task --task-id "$task_id" --status "$status" --result "${result:-}"
}

cmd_query_tasks() {
    local goal_id="" status="" include_executions="false"

    while [[ $# -gt 0 ]]; do
        case $1 in
            --goal-id) goal_id="$2"; shift 2 ;;
            --status) status="$2"; shift 2 ;;
            --include-executions) include_executions="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    local where_clause="TRUE"
    if [[ -n "$goal_id" ]]; then
        where_clause="g.id = '$goal_id'"
    fi

    local status_filter=""
    if [[ -n "$status" ]]; then
        IFS=',' read -ra status_arr <<< "$status"
        local status_parts=()
        for s in "${status_arr[@]}"; do
            s=$(echo "$s" | xargs)
            if [[ "$s" == "awaiting_*" ]]; then
                status_parts+=("t.status STARTS WITH 'awaiting_'")
            else
                status_parts+=("t.status = '$s'")
            fi
        done
        status_filter="AND (${status_parts[*]})"
    fi

    local cypher="
        MATCH (g:Goal)-[:CONTAINS]->(t:Task)
        WHERE $where_clause $status_filter
        OPTIONAL MATCH (t)-[dep:DEPENDS_ON]->(pre:Task)
        WITH t, g, collect(pre.id) as depends_on
        RETURN t.id, t.title, t.agent, t.cron_expr, t.status,
               t.priority, t.progress, t.success_count, t.failure_count,
               t.summary, t.scheduler_id, t.session_id,
               t.interruption_type, t.interruption_context, t.verification_note,
               depends_on, g.title as goal_title,
               t.created_at, t.updated_at
        ORDER BY t.updated_at DESC
    "

    run_cypher "$cypher"
}

cmd_query_by_status() {
    local status=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --status) status="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$status" ]]; then
        echo "❌ Missing required parameter: --status"
        exit 1
    fi

    local cypher="
        MATCH (t:Task {status: '$status'})
        OPTIONAL MATCH (g:Goal)-[:CONTAINS]->(t)
        OPTIONAL MATCH (p:Project)-[:HAS_GOAL]->(g)
        RETURN t.id, t.title, t.agent, t.status, t.progress,
               t.success_count, t.failure_count, t.updated_at,
               g.title as goal_title, p.name as project_name
        ORDER BY t.updated_at DESC
        LIMIT 50
    "

    run_cypher "$cypher"
}

# ====== Relationship Management Commands ======

cmd_add_dependency() {
    local task_id="" depends_on=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --task-id) task_id="$2"; shift 2 ;;
            --depends-on) depends_on="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$task_id" || -z "$depends_on" ]]; then
        echo "❌ Missing required parameters: --task-id, --depends-on"
        exit 1
    fi

    local cypher="
        MATCH (t:Task {id: '$task_id'})
        MATCH (pre:Task {id: '$depends_on'})
        MERGE (t)-[:DEPENDS_ON]->(pre)
        RETURN t.id as task, pre.id as depends_on
    "

    run_cypher "$cypher"
}

cmd_remove_dependency() {
    local task_id="" depends_on=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --task-id) task_id="$2"; shift 2 ;;
            --depends-on) depends_on="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$task_id" || -z "$depends_on" ]]; then
        echo "❌ Missing required parameters: --task-id, --depends-on"
        exit 1
    fi

    local cypher="
        MATCH (t:Task {id: '$task_id'})-[dep:DEPENDS_ON]->(pre:Task {id: '$depends_on'})
        DELETE dep
        RETURN 'dependency removed' as result
    "

    run_cypher "$cypher"
}

# ====== Query Tool Commands ======

cmd_get_task() {
    local task_id=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --task-id) task_id="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$task_id" ]]; then
        echo "❌ Missing required parameter: --task-id"
        exit 1
    fi

    local cypher="
        MATCH (t:Task {id: '$task_id'})
        OPTIONAL MATCH (t)-[dep:DEPENDS_ON]->(pre:Task)
        OPTIONAL MATCH (g:Goal)-[:CONTAINS]->(t)
        OPTIONAL MATCH (t)-[:HAS_EXECUTION]->(e:Execution)
        WITH t, g, pre,
             collect(DISTINCT pre.id) as dependencies,
             collect(e) as executions
        RETURN t, g.title as goal_title, dependencies, executions
    "

    run_cypher "$cypher"
}

cmd_get_goal() {
    local goal_id=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --goal-id) goal_id="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$goal_id" ]]; then
        echo "❌ Missing required parameter: --goal-id"
        exit 1
    fi

    local cypher="
        MATCH (g:Goal {id: '$goal_id'})
        OPTIONAL MATCH (p:Project)-[:HAS_GOAL]->(g)
        OPTIONAL MATCH (g)-[:CONTAINS]->(t:Task)
        RETURN g, p.name as project_name,
               count(t) as total_tasks,
               count(CASE WHEN t.status = 'completed' THEN 1 END) as completed_tasks
    "

    run_cypher "$cypher"
}

cmd_progress_report() {
    local project_id=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --project-id) project_id="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$project_id" ]]; then
        echo "❌ Missing required parameter: --project-id"
        exit 1
    fi

    local cypher="
        MATCH (p:Project {id: '$project_id'})-[:HAS_GOAL]->(g:Goal)
        OPTIONAL MATCH (g)-[:CONTAINS]->(t:Task)
        WITH p, g,
             count(t) as total_tasks,
             count(CASE WHEN t.status = 'completed' THEN 1 END) as completed,
             count(CASE WHEN t.status = 'in_progress' THEN 1 END) as in_progress,
             count(CASE WHEN t.status = 'pending' THEN 1 END) as pending,
             count(CASE WHEN t.status = 'failed' THEN 1 END) as failed,
             sum(t.success_count) as total_success,
             sum(t.failure_count) as total_failures
        RETURN p.id, p.name, p.status, p.progress,
               collect({
                 goal_id: g.id,
                 goal_title: g.title,
                 goal_weight: g.weight,
                 goal_status: g.status,
                 goal_progress: g.progress,
                 tasks: total_tasks,
                 completed: completed,
                 in_progress: in_progress,
                 pending: pending,
                 failed: failed,
                 successes: total_success,
                 failures: total_failures
               }) as goals_data
    "

    run_cypher "$cypher"
}

# ====== Session Management Commands ======

cmd_register_session() {
    local task_id="" agent="" session_status="" created_by=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --task-id) task_id="$2"; shift 2 ;;
            --agent) agent="$2"; shift 2 ;;
            --session-status) session_status="$2"; shift 2 ;;
            --created-by) created_by="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$task_id" || -z "$agent" ]]; then
        echo "❌ Missing required parameters: --task-id, --agent"
        exit 1
    fi

    local id="sess-$(generate_uuid)"
    local now=$(get_timestamp)

    local cypher="
        MATCH (t:Task {id: '$task_id'})
        CREATE (s:Session {
            id: '$id',
            task_id: '$task_id',
            agent: '${agent}',
            status: '${session_status:-initialized}',
            created_by: '${created_by:-system}',
            interruption_type: NULL,
            interruption_context: NULL,
            resolution: NULL,
            resolved_at: NULL,
            replacement_session_id: NULL,
            loss_reason: NULL,
            timeout_reason: NULL,
            created_at: '$now',
            updated_at: '$now'
        })
        CREATE (t)-[:HAS_SESSION]->(s)
        SET t.session_id = '$id', t.updated_at = '$now'
        RETURN s.id as session_id, s.status as status, t.id as task_id, t.title as task_title
    "

    run_cypher "$cypher"
}

cmd_get_session() {
    local session_id=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --session-id) session_id="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$session_id" ]]; then
        echo "❌ Missing required parameter: --session-id"
        exit 1
    fi

    local cypher="
        MATCH (s:Session {id: '$session_id'})
        OPTIONAL MATCH (t:Task)-[:HAS_SESSION]->(s)
        OPTIONAL MATCH (g:Goal)-[:CONTAINS]->(t)
        OPTIONAL MATCH (p:Project)-[:HAS_GOAL]->(g)
        OPTIONAL MATCH (s)-[:HAS_EXECUTION]->(e:Execution)
        WITH s, t, g, p,
             collect(e) as executions
        RETURN s.*, 
               t.id as task_id, t.title as task_title, t.agent as task_agent, t.status as task_status,
               g.title as goal_title, p.name as project_name,
               executions
    "

    run_cypher "$cypher"
}

cmd_update_session() {
    local session_id="" status="" resolution="" resolved_at=""
    local replacement_session_id="" loss_reason="" timeout_reason=""
    local interruption_context=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --session-id) session_id="$2"; shift 2 ;;
            --status) status="$2"; shift 2 ;;
            --resolution) resolution="$2"; shift 2 ;;
            --resolved-at) resolved_at="$2"; shift 2 ;;
            --replacement-session-id) replacement_session_id="$2"; shift 2 ;;
            --loss-reason) loss_reason="$2"; shift 2 ;;
            --timeout-reason) timeout_reason="$2"; shift 2 ;;
            --interruption-context) interruption_context="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$session_id" ]]; then
        echo "❌ Missing required parameter: --session-id"
        exit 1
    fi

    local updates=""
    local now=$(get_timestamp)

    [[ -n "$status" ]] && updates+=", status: '$status'"
    [[ -n "$resolution" ]] && updates+=", resolution: '$resolution'"
    [[ -n "$resolved_at" ]] && updates+=", resolved_at: '$resolved_at'"
    [[ -n "$replacement_session_id" ]] && updates+=", replacement_session_id: '$replacement_session_id'"
    [[ -n "$loss_reason" ]] && updates+=", loss_reason: '$loss_reason'"
    [[ -n "$timeout_reason" ]] && updates+=", timeout_reason: '$timeout_reason'"
    [[ -n "$interruption_context" ]] && updates+=", interruption_context: $interruption_context"
    updates+=" , updated_at: '$now'"

    local cypher="
        MATCH (s:Session {id: '$session_id'})
        SET ${updates#,}
        RETURN s.id, s.status, s.resolution, s.task_id
    "

    run_cypher "$cypher"
}

cmd_query_sessions() {
    local status="" stale_threshold=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --status) status="$2"; shift 2 ;;
            --stale-threshold) stale_threshold="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    local where_parts=()

    if [[ -n "$status" ]]; then
        if [[ "$status" == "awaiting_*" ]]; then
            where_parts+=("s.status STARTS WITH 'awaiting_'")
        else
            IFS=',' read -ra status_arr <<< "$status"
            for s in "${status_arr[@]}"; do
                s=$(echo "$s" | xargs)
                if [[ "$s" == "awaiting_*" ]]; then
                    where_parts+=("s.status STARTS WITH 'awaiting_'")
                else
                    where_parts+=("s.status = '$s'")
                fi
            done
        fi
    fi

    if [[ -n "$stale_threshold" ]]; then
        local threshold_seconds=""
        case "$stale_threshold" in
            *h) threshold_seconds=$(echo "$stale_threshold" | sed 's/h//') ; threshold_seconds=$((threshold_seconds * 3600)) ;;
            *d) threshold_seconds=$(echo "$stale_threshold" | sed 's/d//') ; threshold_seconds=$((threshold_seconds * 86400)) ;;
            *) threshold_seconds=$stale_threshold ;;
        esac
        where_parts+=("s.updated_at < datetime() - duration('P${threshold_seconds}S')")
    fi

    local where_clause=""
    if [[ ${#where_parts[@]} -gt 0 ]]; then
        where_clause="WHERE ${where_parts[*]}"
    fi

    local cypher="
        MATCH (s:Session) $where_clause
        OPTIONAL MATCH (t:Task)-[:HAS_SESSION]->(s)
        OPTIONAL MATCH (g:Goal)-[:CONTAINS]->(t)
        RETURN s.id, s.status, s.interruption_type, s.resolution,
               s.created_at, s.updated_at, s.loss_reason, s.timeout_reason,
               t.id as task_id, t.title as task_title, t.agent as task_agent,
               g.title as goal_title
        ORDER BY s.updated_at ASC
        LIMIT 50
    "

    run_cypher "$cypher"
}

# ====== Task Output Command ======

cmd_get_task_output() {
    local task_id=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --task-id) task_id="$2"; shift 2 ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$task_id" ]]; then
        echo "❌ Missing required parameter: --task-id"
        exit 1
    fi

    local cypher="
        MATCH (t:Task {id: '$task_id'})
        OPTIONAL MATCH (t)-[:HAS_EXECUTION]->(e:Execution)
        OPTIONAL MATCH (t)-[dep:DEPENDS_ON]->(pre:Task)
        WITH t, e, pre
        ORDER BY e.executed_at DESC
        WITH t,
             collect(DISTINCT pre.id) as depends_on,
             collect(e)[0] as latest_execution
        RETURN t.id, t.title, t.agent, t.status, t.summary,
               t.session_id, t.success_count, t.failure_count,
               t.verification_note, t.interruption_type,
               depends_on,
               latest_execution.id as exec_id,
               latest_execution.status as exec_status,
               latest_execution.result as exec_result,
               latest_execution.executed_at as exec_time,
               latest_execution.duration_seconds as exec_duration
    "

    run_cypher "$cypher"
}

# ====== Main Entry Point ======

main() {
    check_gograph

    case "${1:-}" in
        create-project)    shift; cmd_create_project "$@" ;;
        query-project)     shift; cmd_query_project "$@" ;;
        list-projects)     shift; cmd_list_projects "$@" ;;
        update-project)    shift; cmd_update_project "$@" ;;

        create-goal)       shift; cmd_create_goal "$@" ;;
        query-goals)       shift; cmd_query_goals "$@" ;;
        update-goal)       shift; cmd_update_goal "$@" ;;

        create-task)       shift; cmd_create_task "$@" ;;
        update-task)       shift; cmd_update_task "$@" ;;
        record-execution)  shift; cmd_record_execution "$@" ;;
        query-tasks)       shift; cmd_query_tasks "$@" ;;
        query-by-status)   shift; cmd_query_by_status "$@" ;;
        get-task)          shift; cmd_get_task "$@" ;;
        get-task-output)   shift; cmd_get_task_output "$@" ;;

        register-session)  shift; cmd_register_session "$@" ;;
        get-session)       shift; cmd_get_session "$@" ;;
        update-session)    shift; cmd_update_session "$@" ;;
        query-sessions)    shift; cmd_query_sessions "$@" ;;

        add-dependency)    shift; cmd_add_dependency "$@" ;;
        remove-dependency) shift; cmd_remove_dependency "$@" ;;

        get-goal)          shift; cmd_get_goal "$@" ;;
        progress-report)   shift; cmd_progress_report "$@" ;;

        -h|--help|*)       usage ;;
    esac
}

main "$@"
