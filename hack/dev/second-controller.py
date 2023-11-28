#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Description:
# This script will add a second controller to pipelines-as-code
# It's desgied to be used like this:
#
# ko apply -f <(python3 yamlconvert.py LABEL)
#
# Assuming you have a KO_DOCKER_REPO env variable set, ko will deploy the
# second controller image in the target namespace (default: pipelines-as-code).
#
# You can define a few env variables to change the behavior:
# PAC_CONTROLLER_LABEL: the label of the controller
# PAC_CONTROLLER_TARGET_NS: the namespace to deploy the controller to (default:
#                           pipelines-as-code)
# PAC_CONTROLLER_SECRET: the name of the secret to use (default: LABEL-secret)
# PAC_CONTROLLER_CONFIGMAP: the name of the configmap to use (default: LABEL-configmap)
# PAC_CONTROLLER_SMEE_URL: the url to use if you want to deploy a gosmee
#                          container to this controller. If not set, it won't deploy it.
import os
import sys

import yaml

label_name = os.environ.get("PAC_CONTROLLER_LABEL")
if len(sys.argv) > 1:
    label_name = sys.argv[1]
if not label_name:
    sys.stderr.write(
        """"PAC_CONTROLLER_LABEL env or script argument is not set\n
you need to set it to the name of the label you want to use\n"""
    )
    sys.exit(1)
secret_name = os.environ.get("PAC_CONTROLLER_SECRET", f"{label_name}-secret")
configmap_name = os.environ.get("PAC_CONTROLLER_CONFIGMAP", f"{label_name}-configmap")
target_ns = os.environ.get("PAC_CONTROLLER_TARGET_NS", "pipelines-as-code")
smee_url = os.environ.get("PAC_CONTROLLER_SMEE_URL")

controller = {}
with open("config/400-controller.yaml", "r", encoding="utf-8") as f:
    controller = yaml.load(f, Loader=yaml.FullLoader)
    controller["spec"]["selector"]["matchLabels"]["app.kubernetes.io/name"] = (
        label_name + "-controller"
    )
    controller["metadata"]["name"] = label_name + "-controller"
    controller["spec"]["template"]["metadata"]["labels"]["app"] = (
        label_name + "-controller"
    )
    controller["spec"]["template"]["metadata"]["labels"]["app.kubernetes.io/name"] = (
        label_name + "-controller"
    )
    for container in controller["spec"]["template"]["spec"]["containers"]:
        if container["name"] == "pac-controller":
            for env in container["env"]:
                if env["name"] == "PAC_CONTROLLER_LABEL":
                    env["value"] = label_name + "-controller"
                if env["name"] == "PAC_CONTROLLER_SECRET":
                    env["value"] = secret_name
                if env["name"] == "PAC_CONTROLLER_CONFIGMAP":
                    env["value"] = configmap_name

service = {}
with open("config/401-controller-service.yaml", "r", encoding="utf-8") as f:
    service = yaml.load(f, Loader=yaml.FullLoader)
    service["spec"]["selector"]["app.kubernetes.io/name"] = label_name + "-controller"
    service["metadata"]["name"] = label_name + "-controller"
    service["metadata"]["labels"]["app"] = label_name
    service["metadata"]["labels"]["app.kubernetes.io/name"] = label_name + "-controller"

# reencode as yaml to systdout
yaml.dump(controller, sys.stdout, default_flow_style=False)
print("---")
yaml.dump(service, sys.stdout, default_flow_style=False)

if smee_url:
    print(
        f"""---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gosmee-{label_name}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gosmee-{label_name}
  template:
    metadata:
      labels:
        app: gosmee-{label_name}
    spec:
      containers:
        - image: ghcr.io/chmouel/gosmee:main
          imagePullPolicy: Always
          name: gosmee
          args:
            [
              "client",
              "-o",
              "json",
              "--saveDir",
              "/tmp/save",
              "{smee_url}",
              "http://{label_name}-controller.{target_ns}:8080",
            ]
    """
    )
