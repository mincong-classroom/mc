# Lab Session 4 grading rules — proposal

Lab Session 4 ("Kubernetes Networking") was redesigned in the ESIGELEC course
material (`../esigelec/slides/lab-4.md`). The exercises no longer match the L4
rules that were wired into the grader, so this document proposes a new set of
rules and records what this branch implements.

## What changed in the lab

| | Old L4 (previous grader) | New L4 (`lab-4.md`) |
|---|---|---|
| Ex 1 | `L4_HSV` — expose `hello-server` as a Service | Create the `classroom` namespace and list namespaces |
| Ex 2 | `L4_NPT` — switch API Gateway to `NodePort` | Deploy + expose `team-info-server` in `classroom`, validate cross-namespace DNS |
| Ex 3 | `L4_PNS` — create `prod` + `dev` namespaces with the full stack | Route `/api/about` from the API Gateway to `team-info.classroom` |
| Ex 4 | `L4_EML` / `L4_VTQ` — email / vet bonuses | (removed) |

The redesigned lab is about **Kubernetes networking** end to end: namespaces,
`Service` exposure, cross-namespace DNS resolution, and inter-service routing
through the API Gateway. The `hello-server`, `NodePort`, `prod`/`dev`, and the
PetClinic email/vet exercises are gone.

## Proposed rules

Three rules, one per exercise, keeping the existing `L4_<SYMBOL>` convention.

### `L4_NSC` — Namespace Creation Test (Ex 1) — *manual*

Create the `classroom` namespace and list all namespaces. There is nothing
durable to inspect after grading time (the namespace may have been torn down and
the "list" step is evidentiary), so this is graded from the report. Low weight —
it is a warm-up.

### `L4_TIS` — Team Info Server Deployment Test (Ex 2) — *automated*

This is the exercise worth automating, and this branch implements it in
`rules/lab4.go` (`K8sTeamInfoServerRule`). The Team Info Server
([`mincongclassroom/team-info-server`](https://hub.docker.com/r/mincongclassroom/team-info-server))
listens on `8090`, **requires** the `TEAM_ID` env var (it exits on startup when
missing — that missing var is the failure students troubleshoot), and returns a
JSON body with `team`, `members`, `git_repo`, and `docker_repos`. All of that is
machine-checkable.

The rule reuses the existing `kubeApply` / `kubePortForward` / `getHttpContent`
helpers (same pattern as `K8sJavaPodRule`) and scores:

| Weight | Check |
|---|---|
| 0.2 | Manifest committed at `k8s/lab-4/app-team-info.yaml` |
| 0.3 | `team-info` Service reachable and returns a JSON body |
| 0.3 | `team` field (echo of `TEAM_ID`) equals the team name |
| 0.2 | All team members appear in the response |

The **cross-namespace DNS write-up** (querying the Service by short name from
`classroom` vs. by FQDN `team-info.classroom.svc.cluster.local` from `default`)
is a report/comprehension item and stays manual.

### `L4_AGR` — API Gateway About Route Test (Ex 3) — *manual*

Route `/api/about` from the API Gateway to `team-info.classroom` by editing the
`api-gateway-config` ConfigMap — networking only, no code. Deciding this rule
automatically would require the full microservice stack running plus inspecting
the ConfigMap to confirm the info is **not hard-coded**, so it stays manual. An
optional future automation could `curl http://localhost:8080/api/about/` (after a
port-forward to the API Gateway) and assert the JSON `team` matches the team name.

## What this branch changes

- Adds `rules/lab4.go` with the three specs and the automated `L4_TIS` rule.
- Wires the three rules into `Grader` (`NewGrader`, `ListRuleRepresentations`, `GradeL4`).
- Removes the retired specs: deletes `rules/k8s_service.go` (`L4_HSV`, `L4_NPT`)
  and `rules/k8s_namespace.go` (`L4_PNS`), and drops `L4_EML` / `L4_VTQ` from
  `rules/petclinic.go` (the L5 `L4`-unrelated `GAI` spec is kept).
- Updates the `L4` section of `README.md` to match.

## Open questions for review

- **NodePort** — issue [esigelec#180](https://github.com/mincong-h/esigelec/issues/180)
  ("Add back NodePort in L4?") is still open. If NodePort returns to the lab, add
  it back as a new rule rather than reviving `L4_NPT` verbatim.
- **`team-info-server` image tag** — the lab pins `2026.0-rc3`, a release
  candidate (issue [esigelec#178](https://github.com/mincong-h/esigelec/issues/178)).
  The rule matches on the JSON contract, not the tag, so it is unaffected, but the
  lab should move to a stable tag.
- **Stale lab intro** — `lab-4.md` still opens with "multiple environments and
  cross-team collaboration" (issue [esigelec#170](https://github.com/mincong-h/esigelec/issues/170)),
  language from the old design. Not a grader concern, noted for consistency.
- **Should `L4_NSC` be automated?** A `kubectl get namespace classroom` check is
  trivial but the namespace is often gone by grading time; kept manual for now.
