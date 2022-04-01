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

class View {

  constructor(tagName) {
    this.rootEl = document.createElement(tagName === undefined ? 'div' : tagName);
  }

  getRootEl() {
    return this.rootEl;
  }

  setAttribute(k, v) {
    this.rootEl.setAttribute(k, v);
  }

  addEventListener(typ, f) {
    this.rootEl.addEventListener(typ, f)
  }

  show() {
    this.rootEl.style.display = '';
  }

  hide() {
    this.rootEl.style.display = 'none';
  }

  setInvisible() {
    this.rootEl.style.visibility = 'hidden';
  }

  setVisible() {
    this.rootEl.style.visibility = 'visible';
  }

  addClass(className) {
    this.rootEl.classList.add(className);
  }

  removeClass(className) {
    this.rootEl.classList.remove(className);
  }

  onClick(func) {
    this.rootEl.onclick = func;
  }

  appendView(view) {
    this.appendElement(view.rootEl);
  }

  appendText(text) {
    this.appendElement(document.createTextNode(text));
  }

  appendElement(el) {
    this.rootEl.appendChild(el);
  }

  removeElement(el) {
    this.rootEl.removeChild(el);
  }

  removeView(view) {
    this.rootEl.removeChild(view.getRootEl());
  }

  disable() {
    this.rootEl.disabled = true;
  }

  enable() {
    this.rootEl.disabled = false;
  }

  reset() {
    while (this.rootEl.firstChild) {
      this.rootEl.removeChild(this.rootEl.firstChild);
    }
  }

}
