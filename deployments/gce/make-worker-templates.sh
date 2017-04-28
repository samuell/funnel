#!/bin/bash

# This creates a GCE Instance Templates for a Funnel workers.
#
# This expects the "funnel" image family to exist already.

FUNNEL_SERVER='funnel-server:9090'
MACHINE_TYPES="
n1-standard-1
n1-standard-4
"

for mt in $MACHINE_TYPES; do
  NAME="funnel-worker-$mt"
  gcloud compute instance-templates create $NAME \
    --scopes compute-rw,storage-rw \
    --tags funnel \
    --image-family funnel \
    --machine-type $mt \
    --boot-disk-type 'pd-standard' \
    --boot-disk-size '250GB'
done