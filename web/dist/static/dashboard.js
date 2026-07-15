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
      const action = event.target.closest('.modal-action');
      if (!action || action.matches(':disabled')) return;
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
    'telegram-setup',
  ];

  function initDashboardTabs() {
    const tabBar = document.querySelector('.dashboard-tabs');
    if (!tabBar) return;

    const links = tabBar.querySelectorAll('[data-tab-link]');
    const views = DASHBOARD_VIEW_IDS.map((id) => document.getElementById(id)).filter(Boolean);

    function showView(viewID) {
      const targetID = DASHBOARD_VIEW_IDS.includes(viewID) ? viewID : 'schedule-section';
      views.forEach((view) => {
        view.classList.toggle('is-active', view.id === targetID);
      });
      links.forEach((link) => {
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

  document.body.addEventListener('htmx:afterSwap', (event) => {
    const target = event.detail.target;
    if (!(target instanceof HTMLElement) || !target.classList.contains('dashboard-view')) return;
    const activeLink = document.querySelector('[data-tab-link].is-active');
    if (activeLink?.getAttribute('href') === `#${target.id}`) {
      target.classList.add('is-active');
    }
  });

  initDashboardTabs();
})();
