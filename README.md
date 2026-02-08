LFX Antrea Pre-Test – PacketCapture
Overview

This repository contains my implementation for the Antrea PacketCapture pre-test as part of the LFX Mentorship Program.

In addition to implementing packet capture functionality, this work focuses on correctness under real Kubernetes conditions, specifically around pod lifecycle, controller reconciliation, and capture process management.

The primary goal was not just to “produce packet output”, but to ensure that captured data is always semantically correct.
---
Problem Context

The task requires capturing packets from a target pod using tcpdump, orchestrated by a Kubernetes controller.

While the basic implementation is straightforward, real Kubernetes environments introduce challenges such as:

- pod restarts

- reconcile loops

- state reuse

- asynchronous lifecycle events

These conditions can lead to subtle correctness bugs that are not immediately visible from successful command execution alone.
---
