# LFX Antrea Pre-Test – PacketCapture

## Overview

This repository contains my implementation for the **Antrea PacketCapture pre-test** as part of the **LFX Mentorship Program**.

In addition to implementing packet capture functionality, this work focuses on **correctness under real Kubernetes conditions**, particularly around pod lifecycle, controller reconciliation, and capture process management.

The primary goal was not just to *produce packet output*, but to ensure that **captured data is always semantically correct**.

---

## Problem Context

The task requires capturing packets from a target pod using `tcpdump`, orchestrated by a Kubernetes controller.

While the basic implementation is straightforward, real Kubernetes environments introduce challenges such as:

- pod restarts  
- reconcile loops  
- state reuse  
- asynchronous lifecycle events  

These conditions can lead to **subtle correctness bugs** that are not immediately visible from successful command execution alone.

---

## Design Issue Identified: Silent Packet Capture Drift

During testing, I identified a **silent correctness issue** where packet capture could attach to an **unintended pod**, even though the capture process appeared to run successfully.

### Observed behavior

- A `PacketCapture` CR targeted a specific application pod  
- `tcpdump` started successfully  
- However, packets were captured from a **different pod** (observed with CoreDNS)

The system appeared healthy, but the diagnostic data was **incorrect**.

This class of bug is particularly dangerous because it **does not fail loudly**.

---

## Root Cause Analysis

The issue stemmed from two related design gaps:

### 1. Weak coupling between capture state and pod identity

- Pod identity was resolved once and implicitly assumed to remain valid  
- Capture state was reused across reconcile cycles  
- Pod **names** were treated as stable identifiers  

In Kubernetes, pod **UIDs change on recreation**, even when names remain the same.

---

### 2. Capture lifecycle not strictly bound to pod lifecycle

Capture processes could survive:

- pod deletion  
- pod recreation  
- reconcile restarts  

This allowed stale capture state to persist and be reused incorrectly.

Together, these issues allowed packet capture to **silently drift away from the intended pod**.

---

## Key Fix: Binding Capture to Pod UID

To address this, packet capture state is now **strictly bound to the target pod UID** and revalidated on every reconcile.

### Core validation logic

```go
// Validate that the capture is still bound to the same pod instance.
// Pod names may remain stable across restarts, but UIDs do not.
if pc.Status.PodUID != "" && pc.Status.PodUID != string(pod.UID) {
    // Stale capture detected — stop and reinitialize
    stopCapture(pc)

    pc.Status.PodUID = ""
    pc.Status.PodName = ""
}
```
---

### What this guarantees

1. Capture cannot survive pod restarts

2. Pod recreation always triggers explicit cleanup

3. Capture either:

    - runs on the correct pod, or

    - is deterministically restarted

No silent mis-captures.

---

### Additional Hardening

Beyond UID validation, the controller logic was strengthened to ensure:

1. Explicit cleanup on pod deletion

2. Revalidation on every reconcile cycle

3. Capture restart on target changes

4. Output paths scoped per pod instance to prevent reuse

These changes make capture behavior deterministic and lifecycle-safe.

---

### Why This Matters

Packet capture is typically used for debugging production network issues.

Capturing packets from the wrong pod can:

    - mislead operators

    - waste debugging time

    - lead to incorrect conclusions about system behavior

Correctness here is more important than simply producing output.

---

# Quick Start
Prerequisites

- Docker

- kind

- kubectl

- Go (>= 1.21)

- make

---

### Create a Kind Cluster
```
kind create cluster --name packet-capture --config kind-config.yaml
```
---

### Deploy Controller and agent
```
make deploy
```
---

### Create a test Workload
```
kubectl apply -f examples/nginx.yaml
```
---

### Start packet capture
```
kubectl apply -f examples/packetcapture.yaml
```
---

### Verify Capture
```
kubectl get packetcapture
kubectl logs -n packet-capture-system <agent-pod>
```
---

### How to Verify Correctness

1. Deploy the controller and agent

2. Create a PacketCapture CR targeting a pod

3. Restart the target pod

4. Observe:

    - existing capture is stopped

    - pod UID mismatch is detected

    - capture is restarted on the new pod instance

This confirms correct lifecycle coupling.

---

### Repository Structure
```
.
├── controller/            # PacketCapture controller logic
├── agent/                 # tcpdump execution logic
├── artifacts/             # Capture summaries and reproducible outputs
│   ├── .gitkeep
│   ├── capture-files.txt
│   ├── capture-summary.txt
│   └── pods.txt
├── config/                # CRDs and manifests
├── Makefile
└── README.md
```
### Large decoded packet outputs are intentionally excluded from version control, as they are environment-specific and reproducible from capture data.
---

### Lessons Learned

    - Pod names are not stable identifiers

 - Reconcile success does not imply semantic correctness

 - Silent failures are more dangerous than crashes

 - Controller state must always be validated against current cluster reality

 ---

### Conclusion

This work demonstrates not only implementation of packet capture functionality, but also identification and resolution of a real controller correctness issue involving pod identity and lifecycle management.

The resulting design prioritizes correctness, determinism, and operational safety under real Kubernetes conditions.

---

### Most implementations stop at “working output”.
# This implementation ensures the output is correct.
