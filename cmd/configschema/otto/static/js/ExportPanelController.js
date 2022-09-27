
// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

class ExportPanelController {

  constructor(parentView) {
    this.rootView = new ExportPanelView();
    this.rootView.disableButton();
    parentView.appendView(this.rootView);
    this.rootView.onButtonClicked(() => this.handleRenderClick());
  }

  setPipelineTypeProvider(pipelineTypeProvider) {
    this.pipelineTypeProvider = pipelineTypeProvider;
  }

  setReceiverController(receiverController) {
    this.receiverController = receiverController;
  }

  setProcessorController(processorController) {
    this.processorController = processorController;
  }

  setExporterController(exporterController) {
    this.exporterController = exporterController;
  }

  handleRenderClick() {
    this.rootView.setText(buildCollectorYaml(
      this.pipelineTypeProvider.getPipelineType(),
      this.receiverController.getComponentName(),
      this.receiverController.getYaml(),
      this.processorController.getComponentName(),
      this.processorController.getYaml(),
      this.exporterController.getComponentName(),
      this.exporterController.getYaml()
    ));
  }

  pipelineTypeSelected(pipelineType) {
    if (pipelineType === "") {
      this.rootView.disableButton();
    } else {
      this.rootView.enableButton();
    }
  }

  reset() {
  }

}

function buildCollectorYaml(pipelineType, receiverName, receiverYaml, processorName, processorYaml, exporterName, exporterYaml) {
  const indenter = new Indenter('  ');
  let yaml = '';
  yaml += buildReceiverYaml(indenter, receiverName, receiverYaml);
  yaml += buildProcessorYaml(indenter, processorName, processorYaml);
  yaml += buildExporterYaml(indenter, exporterName, exporterYaml);
  yaml += buildServiceYaml(indenter, pipelineType, receiverName, processorName, exporterName);
  return yaml;
}

function buildReceiverYaml(indenter, receiverName, receiverYaml) {
  return buildSingleComponentYaml(indenter, 'receivers', receiverName, receiverYaml);
}

function buildProcessorYaml(indenter, processorName, processorYaml) {
  return buildSingleComponentYaml(indenter, 'processors', processorName, processorYaml);
}

function buildExporterYaml(indenter, processorName, processorYaml) {
  return buildSingleComponentYaml(indenter, 'exporters', processorName, processorYaml);
}

function buildSingleComponentYaml(indenter, componentType, componentName, yaml) {
  if (componentName === "" || yaml === undefined) {
    return "";
  }
  return componentType + ':\n' + buildComponentYaml(indenter, componentName, yaml);
}

function buildComponentYaml(indenter, receiverName, receiverYaml) {
  let out = receiverName + ':\n';
  out += indenter.indent(receiverYaml);
  return indenter.indent(out);
}

function buildServiceYaml(indenter, pipelineType, receiverName, processorName, exporterName) {
  let componentsBlock = 'receivers: [' + receiverName + ']\n';
  if (processorName !== "") {
    componentsBlock += 'processors: [' + processorName + ']\n'
  }
  if (exporterName !== "") {
    componentsBlock += 'exporters: [' + exporterName + ']\n'
  }
  const pipelineTypeBlock = pipelineType + ':\n' + indenter.indent(componentsBlock);
  const pipelinesBlock = 'pipelines:\n' + indenter.indent(pipelineTypeBlock);
  return 'service:\n' + indenter.indent(pipelinesBlock);
}

class Indenter {

  constructor(indentStr) {
    this.indentStr = indentStr;
  }

  indent(text, level) {
    const padding = this.mkPadding(level === undefined ? 1 : level);
    let out = '';
    const lines = text.split(/\r?\n/);
    lines.forEach(line => {
      if (line.length) {
        out += padding + line + '\n';
      }
    });
    return out;
  }

  mkPadding(level) {
    let padding = ''
    for (let i = 0; i < level; i++) {
      padding += this.indentStr;
    }
    return padding;
  }

}
