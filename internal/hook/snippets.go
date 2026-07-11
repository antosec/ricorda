package hook

import "fmt"

// BlockFor returns the managed rc-file block for a shell, or "" when the
// shell is unknown.
func BlockFor(shell string) string {
	body, ok := bodies[shell]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s\n%s\n%s", BeginMark, body, EndMark)
}

// The snippets share a contract: capture the last command with its exit
// code, duration and cwd, then hand it to `ricorda journal add` in a
// fire-and-forget way that never blocks the prompt and never breaks the
// shell if ricorda is missing. Command text and cwd travel base64-encoded
// to sidestep quoting across shells.
var bodies = map[string]string{
	"pwsh": `# ricorda v1 - records each command (exit code, duration, cwd) into the
# local journal. Nothing leaves this machine. Remove: ricorda hook uninstall
if ($null -eq $global:__ricorda_prev_prompt) { $global:__ricorda_prev_prompt = $function:prompt }
function global:prompt {
    $__ok = $?
    $__native = $global:LASTEXITCODE
    try {
        $__h = Get-History -Count 1
        if ($__h -and $__h.Id -ne $global:__ricorda_last_id) {
            $global:__ricorda_last_id = $__h.Id
            $__exit = 0
            if (-not $__ok) {
                $__exit = 1
                if ($null -ne $__native -and $__native -ne 0) { $__exit = $__native }
            }
            $__dur = [int64]($__h.EndExecutionTime - $__h.StartExecutionTime).TotalMilliseconds
            $__cmd64 = [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($__h.CommandLine))
            $__cwd64 = [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes((Get-Location).Path))
            $__psi = New-Object System.Diagnostics.ProcessStartInfo
            $__psi.FileName = "ricorda"
            $__psi.Arguments = "journal add --shell pwsh --exit $__exit --dur-ms $__dur --cmd-b64 $__cmd64 --cwd-b64 $__cwd64"
            $__psi.UseShellExecute = $false
            $__psi.CreateNoWindow = $true
            [void][System.Diagnostics.Process]::Start($__psi)
            if ($__exit -ne 0) {
                try { & ricorda whisper --exit $__exit --cmd-b64 $__cmd64 --cwd-b64 $__cwd64 } catch {}
            }
        }
    } catch {}
    & $global:__ricorda_prev_prompt
}`,

	"bash": `# ricorda v1 - records each command (exit code, duration, cwd) into the
# local journal. Nothing leaves this machine. Remove: ricorda hook uninstall
# Note: uses a DEBUG trap; if you compose your own DEBUG trap, load it first.
__ricorda_preexec() {
    if [ -z "$__ricorda_t0" ] && [ -n "$EPOCHREALTIME" ]; then
        __ricorda_t0=${EPOCHREALTIME//[.,]/}
    fi
}
__ricorda_prompt() {
    local __ex=$?
    local __line
    __line=$(HISTTIMEFORMAT= builtin history 1) || { __ricorda_seeded=1; return 0; }
    if [[ $__line =~ ^[[:space:]]*([0-9]+)[[:space:]]+(.*)$ ]]; then
        local __id=${BASH_REMATCH[1]} __cmd=${BASH_REMATCH[2]}
        # First prompt of the session: seed the id without recording, or the
        # last command of the previous session gets journaled again (bash
        # loads the history file after rc processing).
        if [ -z "$__ricorda_seeded" ]; then
            __ricorda_seeded=1
            __ricorda_last_id=$__id
        elif [ "$__id" != "$__ricorda_last_id" ]; then
            __ricorda_last_id=$__id
            local __dur=0
            if [ -n "$__ricorda_t0" ] && [ -n "$EPOCHREALTIME" ]; then
                local __now=${EPOCHREALTIME//[.,]/}
                __dur=$(( (__now - __ricorda_t0) / 1000 ))
                [ "$__dur" -lt 0 ] && __dur=0
            fi
            if command -v ricorda >/dev/null 2>&1; then
                local __b64
                __b64=$(printf %s "$__cmd" | base64 2>/dev/null | tr -d '\n')
                (ricorda journal add --shell bash --exit "$__ex" --dur-ms "$__dur" --cwd "$PWD" --cmd-b64 "$__b64" >/dev/null 2>&1 &)
                if [ "$__ex" -ne 0 ]; then
                    ricorda whisper --exit "$__ex" --cwd "$PWD" --cmd-b64 "$__b64" 2>/dev/null
                fi
            fi
        fi
    fi
    return 0
}
__ricorda_reset() { __ricorda_t0=; }
trap '__ricorda_preexec' DEBUG
PROMPT_COMMAND="__ricorda_prompt${PROMPT_COMMAND:+;$PROMPT_COMMAND};__ricorda_reset"`,

	"zsh": `# ricorda v1 - records each command (exit code, duration, cwd) into the
# local journal. Nothing leaves this machine. Remove: ricorda hook uninstall
zmodload zsh/datetime 2>/dev/null
autoload -Uz add-zsh-hook
__ricorda_preexec() {
    __ricorda_cmd=$1
    __ricorda_t0=$EPOCHREALTIME
}
__ricorda_precmd() {
    local __ex=$?
    [ -n "$__ricorda_cmd" ] || return 0
    local __dur=0
    if [ -n "$__ricorda_t0" ] && [ -n "$EPOCHREALTIME" ]; then
        __dur=$(( (EPOCHREALTIME - __ricorda_t0) * 1000 ))
        __dur=${__dur%.*}
    fi
    if command -v ricorda >/dev/null 2>&1; then
        local __b64
        __b64=$(printf %s "$__ricorda_cmd" | base64 | tr -d '\n')
        (ricorda journal add --shell zsh --exit "$__ex" --dur-ms "$__dur" --cwd "$PWD" --cmd-b64 "$__b64" >/dev/null 2>&1 &)
        if [ "$__ex" -ne 0 ]; then
            ricorda whisper --exit "$__ex" --cwd "$PWD" --cmd-b64 "$__b64" 2>/dev/null
        fi
    fi
    __ricorda_cmd=
    __ricorda_t0=
}
add-zsh-hook preexec __ricorda_preexec
add-zsh-hook precmd __ricorda_precmd`,

	"fish": `# ricorda v1 - records each command (exit code, duration, cwd) into the
# local journal. Nothing leaves this machine. Remove: ricorda hook uninstall
function __ricorda_postexec --on-event fish_postexec
    set -l __ex $status
    set -l __dur $CMD_DURATION
    if not type -q ricorda
        return
    end
    set -l __b64 (printf %s "$argv[1]" | base64 | tr -d \n)
    ricorda journal add --shell fish --exit $__ex --dur-ms $__dur --cwd $PWD --cmd-b64 $__b64 >/dev/null 2>&1 &
    disown 2>/dev/null
    if test $__ex -ne 0
        ricorda whisper --exit $__ex --cwd $PWD --cmd-b64 $__b64 2>/dev/null
    end
end`,
}

// Shells lists the shells with a managed snippet.
func Shells() []string {
	return []string{"pwsh", "bash", "zsh", "fish"}
}
