// Copyright Splunk
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

class SelectWidget extends View {

  constructor(name) {
    super('select');
    this.addClass('SelectWidget');
    if (name !== undefined) {
      this.setAttribute('name', name);
    }
  }

  addOption(label, value) {
    const opt = document.createElement('option');
    opt.setAttribute('value', value == null ? "" : value);
    opt.appendChild(document.createTextNode(label));
    this.appendElement(opt);
  }

  onSelected(f) {
    this.addEventListener('change', f);
  }

  getValue() {
    return this.getRootEl().value;
  }

}

class DivWidget extends View {

  constructor(className) {
    super();
    this.addClass('DivWidget');
    if (className !== undefined) {
      this.addClass(className);
    }
  }

}

class LabelWidget extends View {

  constructor(text) {
    super();
    this.addClass('LabelWidget');
    this.setTextAlign('center');
    this.appendText(text);
  }

}

class FieldsetWidget extends View {

  constructor(name) {
    super('fieldset');
    this.addClass('FieldsetWidget');
    const legend = document.createElement('legend');
    legend.appendChild(document.createTextNode(name));
    this.appendElement(legend);
  }

}

class ButtonWidget extends View {

  constructor(text) {
    super('input');
    this.addClass('ButtonWidget');
    this.setAttribute('type', 'button');
    this.setAttribute('value', text);
  }

}

class TextInputWidget extends View {

  constructor(name, placeholder, userInput) {
    super('input');
    this.addClass('TextInputWidget');
    this.setAttribute('type', 'text');
    this.setAttribute('name', name);
    if (userInput !== undefined) {
      this.setAttribute('value', userInput);
    }
    if (placeholder !== undefined) {
      this.setAttribute('placeholder', placeholder);
    }
  }

}

class LinkWidget extends View {

  constructor(content, onClick, className) {
    super('a');
    this.addClass('LinkWidget');
    this.addClass(className);
    this.appendElement(document.createTextNode(content));
    this.onClick(onClick);
  }

}

class TextareaWidget extends View {

  constructor() {
    super('textarea');
    this.addClass('TextareaWidget');
  }

  setText(text) {
    if (this.textNode !== undefined) {
      this.removeElement(this.textNode);
    }
    this.textNode = document.createTextNode(text);
    this.appendElement(this.textNode);
  }

  reset() {
    this.setText('');
  }

}

class FormWidget extends View {

  constructor() {
    super('form');
    this.addClass('FormWidget');
  }

  forEachFormElement(f) {
    const formEl = this.getRootEl();
    for (let i = 0; i < formEl.elements.length; i++) {
      f(this.el.elements[i]);
    }
  }

  numFormElements() {
    return this.getRootEl().elements.length;
  }

  getFormElement(i) {
    return this.getRootEl().elements[i];
  }

}

class ImageWidget extends View {

  constructor(src) {
    super('img');
    this.setAttribute('src', src);
  }

}
