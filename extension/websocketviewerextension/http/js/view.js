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

class View {

  constructor(v) {
    if (typeof v === 'object') {
      this.el = v;
    } else {
      this.el = document.createElement(v === undefined ? 'div' : v);
    }
  }

  getRootEl() {
    return this.el;
  }

  setBackgoundColor(v) {
    this.el.style.backgroundColor = v;
  }

  setPadding(v) {
    this.el.style.padding = v;
  }

  setCursor(v) {
    this.el.style.cursor = v;
  }

  setBorderRadius(v) {
    this.el.style.borderRadius = v;
  }

  setHeight(v) {
    this.el.style.height = v;
  }

  setWidth(v) {
    this.el.style.width = v;
  }

  setOverflow(v) {
    this.el.style.overflow = v;
  }

  setColor(v) {
    this.el.style.color = v;
  }

  setFontWeight(v) {
    this.el.style.fontWeight = v;
  }

  setFontSize(v) {
    this.el.style.fontSize = v;
  }

  setBorderColor(v) {
    this.el.style.borderColor = v;
  }

  setBorder(v) {
    this.el.style.border = v;
  }

  setBorderWidth(v) {
    this.el.style.borderWidth = v;
  }

  setBorderStyle(v) {
    this.el.style.borderColor = v;
  }

  setBorderCollapse(v) {
    this.el.style.borderCollapse = v;
  }

  setMargin(v) {
    this.el.style.margin = v;
  }

  setFontFamily(v) {
    this.el.style.fontFamily = v;
  }

  setDisplay(v) {
    this.el.style.display = v;
  }

  setFlexDirection(v) {
    this.el.style.flexDirection = v;
  }

  setAlignItems(v) {
    this.el.style.alignItems = v;
  }

  setGap(v) {
    this.el.style.gap = v;
  }

  setTextAlign(v) {
    this.el.style.textAlign = v;
  }

  setGridTemplateColumns(v) {
    this.el.style.gridTemplateColumns = v;
  }

  setGridtemplateRows(v) {
    this.el.style.gridTemplateRows = v;
  }

  setGridTemplateAreas(v) {
    this.el.style.gridTemplateAreas = v;
  }

  setGridArea(v) {
    this.el.style.gridArea = v;
  }

  setAttribute(k, v) {
    this.el.setAttribute(k, v);
  }

  addEventListener(typ, f) {
    this.el.addEventListener(typ, f)
  }

  show() {
    this.el.style.display = '';
  }

  hide() {
    this.el.style.display = 'none';
  }

  setInvisible() {
    this.el.style.visibility = 'hidden';
  }

  setVisible() {
    this.el.style.visibility = 'visible';
  }

  addClass(className) {
    this.el.classList.add(className);
  }

  removeClass(className) {
    this.el.classList.remove(className);
  }

  onClick(func) {
    this.el.onclick = func;
  }

  appendView(view) {
    this.appendElement(view.el);
  }

  appendText(text) {
    this.appendElement(document.createTextNode(text));
  }

  appendElement(el) {
    this.el.appendChild(el);
  }

  removeElement(el) {
    this.el.removeChild(el);
  }

  removeView(view) {
    this.el.removeChild(view.getRootEl());
  }

  disable() {
    this.el.disabled = true;
  }

  enable() {
    this.el.disabled = false;
  }

  reset() {
    while (this.el.firstChild) {
      this.el.removeChild(this.el.firstChild);
    }
  }

}
