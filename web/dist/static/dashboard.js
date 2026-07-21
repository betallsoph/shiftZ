(function () {
  const TAP_ACTION_DELAY = 200;

  function initInteractiveAuthGrid() {
    const page = document.querySelector('.auth-page');
    if (!page || !window.matchMedia('(hover: hover) and (pointer: fine)').matches) return;

    const cellSize = 50;
    const layer = document.createElement('div');
    layer.className = 'auth-grid-effects';
    layer.setAttribute('aria-hidden', 'true');
    page.prepend(layer);

    let lastCell = '';
    page.addEventListener(
      'pointermove',
      (event) => {
        const x = Math.floor(event.clientX / cellSize) * cellSize;
        const y = Math.floor(event.clientY / cellSize) * cellSize;
        const key = `${x}:${y}`;
        if (key === lastCell) return;
        lastCell = key;

        const cell = document.createElement('span');
        cell.className = 'auth-grid-cell';
        cell.style.transform = `translate(${x}px, ${y}px)`;
        layer.append(cell);
        requestAnimationFrame(() => {
          cell.classList.add('is-active');
          window.setTimeout(() => cell.classList.remove('is-active'), 90);
        });
        window.setTimeout(() => cell.remove(), 1100);
      },
      { passive: true }
    );
    page.addEventListener('pointerleave', () => {
      lastCell = '';
    }, { passive: true });
  }

  initInteractiveAuthGrid();

  function springScaleIn(element, options) {
    const from = options?.from ?? 0.95;
    const to = options?.to ?? 1;
    const stiffness = options?.stiffness ?? 380;
    const damping = options?.damping ?? 14;
    const mass = options?.mass ?? 1;
    const state = { value: from, velocity: 0 };
    let lastTime = null;

    element.style.transformOrigin = 'top center';
    element.style.willChange = 'transform';

    function tick(now) {
      if (lastTime === null) {
        lastTime = now;
        requestAnimationFrame(tick);
        return;
      }
      const dt = Math.min((now - lastTime) / 1000, 0.032);
      lastTime = now;
      const acceleration = (-stiffness * (state.value - to) - damping * state.velocity) / mass;
      state.velocity += acceleration * dt;
      state.value += state.velocity * dt;
      element.style.transform = 'scale(' + state.value + ')';
      if (Math.abs(state.value - to) > 0.001 || Math.abs(state.velocity) > 0.001) {
        requestAnimationFrame(tick);
        return;
      }
      element.style.transform = 'scale(' + to + ')';
      element.style.willChange = '';
    }

    requestAnimationFrame(tick);
  }

  function initAuthPasswordStep() {
    const step = document.querySelector('.auth-password-step');
    if (!step) return;
    requestAnimationFrame(() => {
      springScaleIn(step, { from: 0.95, to: 1, stiffness: 380, damping: 14 });
    });
    const focusTarget =
      step.querySelector('input[name="dashboard_email"]') ||
      step.querySelector('input[name="dashboard_password"]');
    if (focusTarget instanceof HTMLInputElement) {
      focusTarget.focus();
    }
  }

  initAuthPasswordStep();

  function findTapTarget(target) {
    if (!(target instanceof Element)) return null;
    const interactive = target.closest('button, a[href], [role="button"]');
    if (!interactive || interactive.dataset.tapImmediate === 'true') return null;
    if (interactive.closest('[data-tap-zone="plain"]')) return null;
    if (interactive.matches(':disabled, [aria-disabled="true"]')) return null;
    return interactive;
  }

  function bounceTouchTarget(element) {
    element.classList.remove('tap-sink', 'tap-bounce');
    void element.offsetWidth;
    element.classList.add('tap-sink');
    window.setTimeout(() => {
      if (!document.body.contains(element)) return;
      element.classList.remove('tap-sink');
      void element.offsetWidth;
      element.classList.add('tap-bounce');
    }, 100);
  }

  document.addEventListener(
    'pointerdown',
    (event) => {
      if (event.pointerType !== 'touch') return;
      const interactive = findTapTarget(event.target);
      if (interactive) bounceTouchTarget(interactive);
    },
    { passive: true }
  );

  document.addEventListener(
    'pointerover',
    (event) => {
      if (!(event.target instanceof Element)) return;
      const action = event.target.closest('.toolbar-action, .modal-action, .btn');
      if (!action || action.matches(':disabled, [aria-disabled="true"]')) return;
      if (event.relatedTarget instanceof Node && action.contains(event.relatedTarget)) return;
      const bounds = action.getBoundingClientRect();
      action.style.setProperty('--hover-x', `${event.clientX - bounds.left}px`);
      action.style.setProperty('--hover-y', `${event.clientY - bounds.top}px`);
    },
    { passive: true }
  );

  async function copyText(text) {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return;
    }
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.append(textarea);
    textarea.select();
    document.execCommand('copy');
    textarea.remove();
  }

  document.addEventListener('click', async (event) => {
    if (!(event.target instanceof Element)) return;
    const button = event.target.closest('[data-copy-text]');
    if (!(button instanceof HTMLButtonElement)) return;
    const text = button.dataset.copyText;
    if (!text) return;

    const originalLabel = button.textContent;
    try {
      await copyText(text);
      button.textContent = 'Đã sao chép';
    } catch {
      button.textContent = 'Không thể sao chép';
    }
    window.setTimeout(() => {
      button.textContent = originalLabel;
    }, 1800);
  });

  document.addEventListener(
    'click',
    (event) => {
      const interactive = findTapTarget(event.target);
      if (!interactive) return;
      const anchor = interactive instanceof HTMLAnchorElement ? interactive : interactive.closest('a[href]');
      if (!anchor) return;
      if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      if (anchor.target && anchor.target !== '_self') return;
      if (anchor.hasAttribute('download')) return;
      const href = anchor.getAttribute('href');
      if (!href || href.startsWith('#') || href.startsWith('mailto:') || href.startsWith('tel:')) return;
      const url = new URL(href, window.location.href);
      if (url.origin !== window.location.origin) return;
      event.preventDefault();
      window.setTimeout(() => {
        window.location.href = `${url.pathname}${url.search}${url.hash}`;
      }, TAP_ACTION_DELAY);
    },
    true
  );

  document.addEventListener(
    'animationend',
    (event) => {
      if (event.animationName === 'tap-bounce' && event.target instanceof HTMLElement) {
        event.target.classList.remove('tap-bounce');
      }
    },
    { passive: true }
  );

  const DASHBOARD_VIEW_IDS = [
    'schedule-section',
    'employees-panel',
    'shifts-panel',
  ];

  function initDashboardTabs() {
    const tabBar = document.querySelector('.dashboard-tabs');
    if (!tabBar) return;

    function showView(viewID) {
      const targetID = DASHBOARD_VIEW_IDS.includes(viewID) ? viewID : 'schedule-section';
      DASHBOARD_VIEW_IDS.forEach((id) => {
        const view = document.getElementById(id);
        if (view) view.classList.toggle('is-active', view.id === targetID);
      });
      tabBar.querySelectorAll('[data-tab-link]').forEach((link) => {
        const active = link.getAttribute('href') === `#${targetID}`;
        link.classList.toggle('is-active', active);
        if (active) link.setAttribute('aria-current', 'page');
        else link.removeAttribute('aria-current');
      });
      if (window.location.hash !== `#${targetID}`) {
        history.replaceState(null, '', `#${targetID}`);
      }
    }

    tabBar.addEventListener('click', (event) => {
      const link = event.target instanceof Element ? event.target.closest('[data-tab-link]') : null;
      if (!link) return;
      event.preventDefault();
      const viewID = link.getAttribute('href')?.slice(1);
      if (viewID) showView(viewID);
    });

    const initial = window.location.hash.slice(1);
    showView(initial);
  }

  // outerHTML swaps replace the node; re-apply is-active on the live element by id.
  document.body.addEventListener('htmx:afterSwap', (event) => {
    const target = event.detail.target;
    if (!(target instanceof HTMLElement)) return;
    const id = target.id;
    if (!DASHBOARD_VIEW_IDS.includes(id)) return;
    const live = document.getElementById(id);
    if (!live) return;
    const activeLink = document.querySelector('[data-tab-link].is-active');
    if (activeLink?.getAttribute('href') === `#${id}`) {
      live.classList.add('is-active');
    }
    enhanceFormControls(live);
  });

  function closeAllCustomSelects(except) {
    document.querySelectorAll('[data-custom-select].is-open').forEach((field) => {
      if (field !== except) field.classList.remove('is-open');
    });
  }

  function enhanceCustomSelect(field) {
    if (!(field instanceof HTMLElement) || field.dataset.enhanced === 'true') return;
    const native = field.querySelector('[data-custom-select-native]');
    if (!(native instanceof HTMLSelectElement)) return;

    const trigger = document.createElement('button');
    trigger.type = 'button';
    trigger.className = 'custom-select-trigger';
    trigger.setAttribute('aria-haspopup', 'listbox');
    trigger.setAttribute('aria-expanded', 'false');

    const label = document.createElement('span');
    label.className = 'custom-select-label';
    const chevron = document.createElement('span');
    chevron.className = 'custom-select-chevron';
    chevron.setAttribute('aria-hidden', 'true');
    trigger.append(label, chevron);

    const menu = document.createElement('ul');
    menu.className = 'custom-select-menu';
    menu.setAttribute('role', 'listbox');

    function syncFromNative() {
      const selected = native.selectedOptions[0];
      label.textContent = selected ? selected.textContent : '';
      menu.querySelectorAll('.custom-select-option').forEach((btn) => {
        btn.classList.toggle('is-selected', btn.dataset.value === native.value);
      });
    }

    Array.from(native.options).forEach((option) => {
      const item = document.createElement('li');
      const button = document.createElement('button');
      button.type = 'button';
      button.className = 'custom-select-option';
      button.dataset.value = option.value;
      button.textContent = option.textContent;
      button.setAttribute('role', 'option');
      button.addEventListener('click', () => {
        native.value = option.value;
        native.dispatchEvent(new Event('change', { bubbles: true }));
        syncFromNative();
        field.classList.remove('is-open');
        trigger.setAttribute('aria-expanded', 'false');
      });
      button.addEventListener('mouseenter', () => {
        menu.querySelectorAll('.custom-select-option').forEach((btn) => btn.classList.remove('is-active'));
        button.classList.add('is-active');
      });
      item.append(button);
      menu.append(item);
    });

    trigger.addEventListener('click', (event) => {
      event.preventDefault();
      const willOpen = !field.classList.contains('is-open');
      closeAllCustomSelects(field);
      field.classList.toggle('is-open', willOpen);
      trigger.setAttribute('aria-expanded', willOpen ? 'true' : 'false');
    });

    field.append(trigger, menu);
    field.dataset.enhanced = 'true';
    syncFromNative();
  }

  function enhanceFormControls(root) {
    const scope = root instanceof Element ? root : document;
    scope.querySelectorAll('[data-custom-select]').forEach(enhanceCustomSelect);
  }

  document.addEventListener('click', (event) => {
    if (!(event.target instanceof Element)) return;
    if (event.target.closest('[data-custom-select]')) return;
    closeAllCustomSelects();
  });

  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') closeAllCustomSelects();
  });

  document.addEventListener('click', (event) => {
    if (!(event.target instanceof Element)) return;
    const stepBtn = event.target.closest('[data-number-step]');
    if (!(stepBtn instanceof HTMLButtonElement)) return;
    const field = stepBtn.closest('[data-number-field]');
    const input = field?.querySelector('input[type="number"]');
    if (!(input instanceof HTMLInputElement)) return;
    event.preventDefault();
    const delta = Number(stepBtn.dataset.numberStep || '0');
    const min = input.min === '' ? null : Number(input.min);
    const max = input.max === '' ? null : Number(input.max);
    const current = input.value === '' ? 0 : Number(input.value);
    let next = (Number.isFinite(current) ? current : 0) + delta;
    if (min !== null && Number.isFinite(min)) next = Math.max(min, next);
    if (max !== null && Number.isFinite(max)) next = Math.min(max, next);
    input.value = String(next);
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new Event('change', { bubbles: true }));
  });

  enhanceFormControls(document);
  initDashboardTabs();
})();
