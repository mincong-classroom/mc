---
commit: d5c907ade0cd2cfd6be09977018ff972e0efa7c6
mc: 0a1b2c3d4e5f
rules:
  L1_BLD:
    screenshots: 1
  L1_TAG:
    images: "1.0.0 0f3e5d6c7b8a, 1.0.1 1a2b3c4d5e6f"
    screenshots: 0
---

# Lab 1: Build an Image, Team red

The facts of each rule, for the teacher: what the team wrote in its report, the files of its repository, and the evidence that the evaluator made. Facts, not a score. `mc eval gather-facts` generates this file: the evaluator writes in [`lab-1.evidence/`](lab-1.evidence/), and `mc eval gather-facts` runs again.

- Commit evaluated: `d5c907a`, the head of main at the deadline (Mon 4 Mar 2030, 08:30 (UTC+01:00)), pushed on Mon 4 Mar 2030, 07:30 (UTC+01:00)
- Gathered by `mc` at `0a1b2c3d4e5f`
- Pushed after the deadline: 1 push, the last one on Mon 4 Mar 2030, 09:30 (UTC+01:00), up to `79add73`: not in this evaluation
- Report of the team: [`docs/lab-1.md`](lab-1.md), 3 lines of the team

| Rule | Exercise | Name | In the report | In the repository | By the evaluator |
|---|---|---|---|---|---|
| [`L1_BLD`](#l1_bld-build-test-ex-1) | 1 | Build Test | 3 lines | 3 files | nothing yet |
| [`L1_TAG`](#l1_tag-publish-test-ex-2) | 2 | Publish Test | 0 lines | 0 files | 1 image on Docker Hub |

## `L1_BLD` Build Test (Ex 1)

Asked: file, the Dockerfile; command, the command docker build.

### In the Report

[Exercise 1: Build](lab-1.md#L7), lines 7 to 19: 3 lines of the team.

```sh
# TODO: enter the docker command here
docker build -t sample/app-red:1.0.0 app
```

We built it on a Mac, see [the logs](../logs/build.txt).

![The build](assets/build.png)

### In the Repository

#### [`app/Dockerfile`](../app/Dockerfile)

A file of the rule, changed since the template: 5 lines added or removed, of 4 lines.

```diff
@@ -1 +1,4 @@
-# TODO: write the Dockerfile
+FROM eclipse-temurin:21
+COPY target/app.jar /app.jar
+EXPOSE 8080
+CMD ["java", "-jar", "/app.jar"]
```

#### [`app/index.html`](../app/index.html)

A file of the rule, changed since the template: 2 lines added or removed, of 101 lines.

```diff
@@ -49,5 +49,5 @@
 <p>The clinic.</p>
 <p>The clinic.</p>
-<footer>Team</footer>
+<footer>Team Red</footer>
 <p>The clinic.</p>
 <p>The clinic.</p>
```

#### [`logs/build.txt`](../logs/build.txt)

Linked from the report, new: 2 lines.

```text
#1 [internal] load build definition
#2 DONE 0.1s
```

### By the Evaluator

Nothing yet: the evaluator writes its notes in [`lab-1.evidence/L1_BLD.md`](lab-1.evidence/L1_BLD.md), and saves its screenshots and logs as `lab-1.evidence/l1-bld-*`.

## `L1_TAG` Publish Test (Ex 2)

Asked: image, the image 1.0.0.

### In the Report

[Exercise 2: Publish](lab-1.md#L20), lines 20 to 27: no line of the team, only the statement.

### In the Repository

No file of the rule, and the report links to none.

### By the Evaluator

On Docker Hub, read by `mc eval gather-facts`:

| Image | Last pushed | Platforms | Digest |
|---|---|---|---|
| `sample/app-red:1.0.0` | Mon 4 Mar 2030, 06:30 (UTC+01:00) | linux/amd64 | `0f3e5d6c7b8a` |
| `sample/app-red:1.0.1`, a patch that counts for it | Mon 4 Mar 2030, 10:30 (UTC+01:00), after the deadline | linux/amd64 | `1a2b3c4d5e6f` |

## Other Files of the Team

The files that the team added or changed for this lab, or outside of the labs, which no rule names and the report doesn't link.

#### [`.DS_Store`](../.DS_Store)

Binary: 8 bytes. Not expected in a repository.
