// Common JS for simple UI behaviours (offline)
(function(){
  // Global Namespace
  const HostBerry = window.HostBerry || {};

  async function performLogout(event){
    if(event){ event.preventDefault(); }
    try{
      await HostBerry.apiRequest('/api/v1/auth/logout', { method: 'POST' });
    }catch(_e){
      // ignore errors to ensure client-side logout proceeds
    }finally{
      localStorage.removeItem('access_token');
      localStorage.removeItem('user_info');
      if(HostBerry.showAlert){
        HostBerry.showAlert('info', HostBerry.t ? HostBerry.t('auth.logout_success', 'Logout successful') : 'Logout successful');
      }
      window.location.href = '/login';
    }
  }

  async function performLogoutAll(event){
    if(event){ event.preventDefault(); }
    const confirmMsg = HostBerry.t
      ? HostBerry.t('auth.logout_all_confirm', 'Are you sure you want to close all active sessions on all devices?')
      : 'Are you sure you want to close all active sessions on all devices?';
    if(!confirm(confirmMsg)) return;

    try{
      await HostBerry.apiRequest('/api/v1/auth/logout-all', { method: 'POST' });
      if(HostBerry.showAlert){
        HostBerry.showAlert('info', HostBerry.t ? HostBerry.t('auth.logout_all_success', 'Logged out from all devices') : 'Logged out from all devices');
      }
    }catch(_e){
      // ignore errors to ensure client-side logout proceeds
    }finally{
      localStorage.removeItem('access_token');
      localStorage.removeItem('user_info');
      window.location.href = '/login';
    }
  }

  async function performRestart(event){
    if(event){ event.preventDefault(); }
    const confirmMsg = HostBerry.t ? HostBerry.t('system.restart_confirm', 'Are you sure you want to restart the system? This will disconnect all users.') : 'Are you sure you want to restart the system? This will disconnect all users.';
    if(!confirm(confirmMsg)) return;
    
    try{
      if(HostBerry.showAlert){
        HostBerry.showAlert('warning', HostBerry.t ? HostBerry.t('system.restarting', 'Restarting system...') : 'Restarting system...');
      }
      await HostBerry.apiRequest('/api/v1/system/restart', { method: 'POST' });
      if(HostBerry.showAlert){
        HostBerry.showAlert('info', HostBerry.t ? HostBerry.t('system.restart_pending', 'Restart command sent') : 'Restart command sent');
      }
      setTimeout(function(){ window.location.reload(); }, 5000);
    }catch(error){
      console.error('Restart failed', error);
      if(HostBerry.showAlert){
        HostBerry.showAlert('danger', HostBerry.t ? HostBerry.t('system.restart_error', 'Unable to restart system') : 'Unable to restart system');
      }
    }
  }

  async function performShutdown(event){
    if(event){ event.preventDefault(); }
    const confirmMsg = HostBerry.t ? HostBerry.t('system.shutdown_confirm', 'Are you sure you want to shutdown the system? This will disconnect all users.') : 'Are you sure you want to shutdown the system? This will disconnect all users.';
    if(!confirm(confirmMsg)) return;
    
    const doubleConfirm = HostBerry.t ? HostBerry.t('system.shutdown_double_confirm', 'Type SHUTDOWN to confirm') : 'Type SHUTDOWN to confirm';
    const userInput = prompt(doubleConfirm);
    if(userInput !== 'SHUTDOWN') return;
    
    try{
      if(HostBerry.showAlert){
        HostBerry.showAlert('warning', HostBerry.t ? HostBerry.t('system.shutting_down', 'Shutting down system...') : 'Shutting down system...');
      }
      await HostBerry.apiRequest('/api/v1/system/shutdown', { method: 'POST' });
      if(HostBerry.showAlert){
        HostBerry.showAlert('info', HostBerry.t ? HostBerry.t('system.shutdown_pending', 'Shutdown command sent') : 'Shutdown command sent');
      }
    }catch(error){
      console.error('Shutdown failed', error);
      if(HostBerry.showAlert){
        HostBerry.showAlert('danger', HostBerry.t ? HostBerry.t('system.shutdown_error', 'Unable to shutdown system') : 'Unable to shutdown system');
      }
    }
  }

  // Simple Dropdown without Bootstrap
  document.addEventListener('click', function(e){
    const toggle = e.target.closest('.dropdown-toggle');
    const inDropdown = e.target.closest('.dropdown');
    const isDropdownItem = e.target.closest('.dropdown-item');
    
    // Si se hace clic en un item del dropdown, no cerrar el dropdown
    if(isDropdownItem && !isDropdownItem.hasAttribute('data-action')){
      return;
    }
    
    document.querySelectorAll('.dropdown').forEach(function(d){
      if(!inDropdown || d !== inDropdown) d.classList.remove('show');
    });
    if(toggle){
      e.preventDefault();
      e.stopPropagation();
      const dd = toggle.closest('.dropdown');
      if(dd) dd.classList.toggle('show');
    }
  });

  // Navbar Toggler (Hamburger)
  document.addEventListener('click', function(e){
    const toggler = e.target.closest('.navbar-toggler');
    if(toggler){
        const targetId = toggler.getAttribute('data-bs-target');
        if(targetId){
            const target = document.querySelector(targetId);
            if(target){
                target.classList.toggle('show');
                const expanded = target.classList.contains('show');
                toggler.setAttribute('aria-expanded', expanded);
            }
        }
    }
  });

  // Cache de traducciones (se rellena desde DOM o por API)
  window._hbTranslations = window._hbTranslations || {};

  function loadTranslations(){
    try{
      const el = document.getElementById('i18n-json');
      if(el){
        window._hbTranslations = JSON.parse(el.textContent || el.innerText || '{}');
        return window._hbTranslations;
      }
      return window._hbTranslations;
    }catch(_e){
      return window._hbTranslations;
    }
  }

  function getTranslations(){ return window._hbTranslations || {}; }

  (function initTranslations(){
    const el = document.getElementById('i18n-json');
    if(el){
      try{ window._hbTranslations = JSON.parse(el.textContent || el.innerText || '{}'); }catch(_e){}
      return;
    }
    const lang = (document.documentElement && document.documentElement.getAttribute('lang')) || 'es';
    fetch('/api/v1/translations/' + lang)
      .then(function(r){ return r.ok ? r.json() : {}; })
      .then(function(data){ if(data && typeof data === 'object') window._hbTranslations = data; })
      .catch(function(){});
  })();

  loadTranslations();

  // t: nested access to keys 'a.b.c'
  function t(key, defaultValue){
    if(!key) return defaultValue || '';
    const parts = String(key).split('.');
    let cur = getTranslations();
    for(const part of parts){
      if(cur && Object.prototype.hasOwnProperty.call(cur, part)){
        cur = cur[part];
      }else{
        return defaultValue || key;
      }
    }
    return typeof cur === 'string' ? cur : (defaultValue || key);
  }

  // Tiempo hasta auto-cierre de notificaciones flotantes (ms)
  var HB_ALERT_AUTO_DISMISS_MS = 8000;

  function isAuthNotificationPage() {
    const page = document.body && document.body.getAttribute ? document.body.getAttribute('data-page') : '';
    return page === 'login' || page === 'first_login';
  }

  function getAuthAlertContainer() {
    const id = 'hb-auth-alert-container';
    let c = document.getElementById(id);
    if (c) return c;
    c = document.createElement('div');
    c.id = id;
    c.style.position = 'fixed';
    c.style.top = '20px';
    c.style.left = '50%';
    c.style.transform = 'translateX(-50%)';
    c.style.zIndex = '10050';
    c.style.width = 'min(92vw, 640px)';
    c.style.pointerEvents = 'none';
    document.body.appendChild(c);
    return c;
  }

  /** Cierra banner in-place (timer + ocultar). */
  function dismissTransientAlert(bannerEl) {
    if (!bannerEl) return;
    if (bannerEl._hbDismissTimer) {
      clearTimeout(bannerEl._hbDismissTimer);
      bannerEl._hbDismissTimer = null;
    }
    bannerEl.classList.add('d-none');
    bannerEl.style.display = 'none';
  }

  /**
   * Alerta en el propio documento: botón X y cierre automático a los 8 s.
   * @param {object} [opts] restartTimer: false = no reiniciar cuenta si ya tenía X (p. ej. polls periódicos).
   */
  function attachTransientAlert(bannerEl, opts) {
    opts = opts || {};
    if (!bannerEl) return;
    if (isAuthNotificationPage() && !bannerEl.closest('#hb-alert-container')) {
      const authContainer = getAuthAlertContainer();
      if (!bannerEl.closest('#hb-auth-alert-container')) {
        authContainer.appendChild(bannerEl);
      }
      bannerEl.style.position = 'relative';
      bannerEl.style.left = '';
      bannerEl.style.right = '';
      bannerEl.style.top = '';
      bannerEl.style.transform = '';
      bannerEl.style.marginBottom = '10px';
      bannerEl.style.pointerEvents = 'auto';
      bannerEl.style.width = '100%';
    }
    const hadCloseBefore = !!bannerEl.querySelector('.hb-transient-alert-close');
    if (!hadCloseBefore) {
      bannerEl.classList.add('alert-dismissible', 'd-flex', 'align-items-center', 'gap-2', 'flex-wrap');
      const wrap = document.createElement('div');
      wrap.className = 'flex-grow-1';
      while (bannerEl.firstChild) {
        wrap.appendChild(bannerEl.firstChild);
      }
      const closeBtn = document.createElement('button');
      closeBtn.type = 'button';
      closeBtn.className = 'btn-close flex-shrink-0 hb-transient-alert-close';
      closeBtn.setAttribute('aria-label', t('common.close', 'Close'));
      bannerEl.appendChild(wrap);
      bannerEl.appendChild(closeBtn);
      closeBtn.addEventListener('click', function (e) {
        e.preventDefault();
        dismissTransientAlert(bannerEl);
      });
    }
    if (hadCloseBefore && opts.restartTimer === false) {
      return;
    }
    if (bannerEl._hbDismissTimer) {
      clearTimeout(bannerEl._hbDismissTimer);
      bannerEl._hbDismissTimer = null;
    }
    bannerEl._hbDismissTimer = setTimeout(function () {
      dismissTransientAlert(bannerEl);
    }, HB_ALERT_AUTO_DISMISS_MS);
  }

  /** ¿El .alert se ve en pantalla? (respeta padres ocultos) */
  function isHbPageAlertVisible(el) {
    if (!el || !el.classList || !el.classList.contains('alert')) return false;
    if (el.classList.contains('d-none')) return false;
    try {
      const r = el.getClientRects();
      return !!(r && r.length > 0);
    } catch (_e) {
      return false;
    }
  }

  /** Aplica X + 8 s a todos los .alert visibles de la página (excepto contenedor flotante). */
  function scanVisibleAlerts() {
    document.querySelectorAll('.alert').forEach(function (el) {
      if (el.closest('#hb-alert-container')) return;
      if (el.getAttribute('data-hb-no-auto-dismiss') === 'true') return;
      if (!isHbPageAlertVisible(el)) return;
      const isFirst = !el.querySelector('.hb-transient-alert-close');
      attachTransientAlert(el, { restartTimer: isFirst });
    });
  }

  // Floating alert top right: auto-cierra a los 8s y botón X manual
  function showAlert(type, message){
    const containerId = 'hb-alert-container';
    let container = document.getElementById(containerId);
    if(!container){
      container = document.createElement('div');
      container.id = containerId;
      container.style.position = 'fixed';
      container.style.zIndex = '9999';
      document.body.appendChild(container);
    }
    container.style.top = '20px';
    container.style.left = '50%';
    container.style.transform = 'translateX(-50%)';
    container.style.right = 'auto';
    container.style.width = 'min(92vw, 640px)';
    container.style.maxWidth = '92vw';
    const alertEl = document.createElement('div');
    alertEl.className = 'alert alert-' + (type || 'info') + ' alert-dismissible fade show shadow d-flex align-items-center';
    alertEl.setAttribute('role', 'alert');
    alertEl.style.marginBottom = '10px';

    const msgSpan = document.createElement('span');
    msgSpan.className = 'flex-grow-1 me-2';
    msgSpan.textContent = message || '';

    const closeBtn = document.createElement('button');
    closeBtn.type = 'button';
    closeBtn.className = 'btn-close flex-shrink-0';
    closeBtn.setAttribute('aria-label', t('common.close', 'Close'));

    var hideTimer = null;
    function dismissAlert(){
      if(hideTimer != null){
        clearTimeout(hideTimer);
        hideTimer = null;
      }
      if(alertEl && alertEl.parentNode){
        alertEl.parentNode.removeChild(alertEl);
      }
    }

    closeBtn.addEventListener('click', function(e){
      e.preventDefault();
      dismissAlert();
    });

    alertEl.appendChild(msgSpan);
    alertEl.appendChild(closeBtn);
    container.appendChild(alertEl);

    hideTimer = setTimeout(dismissAlert, HB_ALERT_AUTO_DISMISS_MS);
  }

  // Fetch wrapper with JSON y detección básica de 401/403
  async function apiRequest(url, options){
    const opts = Object.assign({ method: 'GET', headers: {} }, options || {});
    // Importante: incluir cookies en peticiones al mismo origen (para auth por cookie HTTPOnly).
    // Si no, el wizard/otros endpoints pueden devolver 401 y no mostrar el estado.
    if (!('credentials' in opts)) opts.credentials = 'include';
    const headers = new Headers(opts.headers);
    
    // JSON body handling
    if(opts.body && typeof opts.body === 'object' && !(opts.body instanceof FormData)){
      if(!headers.has('Content-Type')){
        headers.set('Content-Type', 'application/json');
      }
      opts.body = JSON.stringify(opts.body);
    }
    
    opts.headers = headers;
    // Permitir pasar un elemento origen para manejo de errores (por ejemplo, desactivar/reaccionar ante 403)
    const sourceElement = opts.sourceElement || null;
    if(sourceElement){
      delete opts.sourceElement;
    }
    try {
      const resp = await fetch(url, opts);
      if(resp.status === 401 && !url.includes('/auth/login')){
        // Auto logout on unauthorized
        // Pero NO redirigir inmediatamente si es una operación que puede causar pérdida temporal de conexión
        // o si es un error de red (no un error real de autenticación)
        const isNetworkOperation = url.includes('/wifi/connect') || 
                                   url.includes('/network/') || 
                                   url.includes('/system/network');
        
        if(isNetworkOperation){
          console.warn('401 durante operación de red - puede ser pérdida temporal de conexión');
          // No redirigir inmediatamente, dejar que el código de manejo de errores lo haga
          // después de verificar si es un error real o temporal
        } else {
          // Solo cerrar sesión si NO es un error de red y es un 401 real
          // Verificar que realmente es un error de autenticación y no un error de red
          try {
            const errorData = await resp.clone().json().catch(() => ({}));
            const errorMsg = String(errorData.error || '');
            const errorCode = errorData.code || '';
            
            // Si es "Usuario no encontrado", puede ser un problema temporal - no cerrar sesión
            if(errorCode === 'USER_NOT_FOUND' || 
               errorMsg.toLowerCase().includes('usuario no encontrado') ||
               errorMsg.toLowerCase().includes('user not found')){
              console.warn('Usuario no encontrado durante operación - puede ser problema temporal');
              // No cerrar sesión, dejar que el código de manejo de errores específico lo maneje
              return resp;
            }
            
            // Si el mensaje indica que es un error de token/autenticación real, cerrar sesión
            if(errorMsg.includes('token') || errorMsg.includes('Token') || 
               errorMsg.includes('autorizado') || errorMsg.includes('authorized') ||
               errorMsg.includes('expirado') || errorMsg.includes('expired')){
              window.location.href = '/login?error=session_expired';
            }
          } catch(_e) {
            // Si no se puede parsear el error, asumir que es un error de autenticación real
            window.location.href = '/login?error=session_expired';
          }
        }
      }

      // Manejo genérico de 403 (permisos insuficientes)
      if (resp.status === 403) {
        let msg = t('errors.forbidden', 'Permisos insuficientes para realizar esta acción.');
        try{
          const data = await resp.clone().json().catch(() => ({}));
          if(data && typeof data.error === 'string' && data.error.trim() !== ''){
            msg = data.error;
          }
        }catch(_e){}

        if (HostBerry.showAlert) {
          HostBerry.showAlert('warning', msg);
        }

        // Si disponemos de un elemento origen, añadir un indicador visual
        if (sourceElement) {
          try{
            sourceElement.classList.add('disabled');
            sourceElement.setAttribute('aria-disabled', 'true');
          }catch(_e){}
        }
      }

      return resp;
    } catch (e) {
      console.error('API Request failed:', e);
      // No cerrar sesión por errores de red - podría ser temporal
      throw e;
    }
  }

  // Timezone/locale helpers (timezone guardado en Settings y servido por backend)
  function getServerTimezone(){
    return (window.HostBerryServerSettings && window.HostBerryServerSettings.timezone) ? window.HostBerryServerSettings.timezone : 'UTC';
  }

  function getServerLanguage(){
    const lang = (document.documentElement && document.documentElement.lang) ? document.documentElement.lang : 'en';
    return (lang === 'es') ? 'es' : 'en';
  }

  function formatTime(date, options){
    const tz = getServerTimezone();
    const lang = getServerLanguage();
    const locale = (lang === 'es') ? 'es-ES' : 'en-US';
    const d = (date instanceof Date) ? date : new Date(date);
    const fmtOpts = Object.assign({ hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit', timeZone: tz }, options || {});
    try{
      return new Intl.DateTimeFormat(locale, fmtOpts).format(d);
    }catch(_e){
      // Fallback si Intl/timeZone no está disponible
      return d.toLocaleTimeString(locale, { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
    }
  }

  function formatUptime(seconds){
    if (!Number.isFinite(seconds) || seconds < 0) return '--';
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    if (days > 0) return days + 'd ' + hours + 'h ' + minutes + 'm';
    if (hours > 0) return hours + 'h ' + minutes + 'm';
    return minutes + 'm';
  }

  function escapeHtml(text){
    if (text == null) return '';
    const div = document.createElement('div');
    div.textContent = String(text);
    return div.innerHTML;
  }

  function formatBytes(bytes, decimals){
    if (bytes === 0) return '0 B';
    if (!bytes || !Number.isFinite(bytes) || bytes < 0) return '0 B';
    const k = 1024;
    const dm = (decimals !== undefined && decimals >= 0) ? decimals : 2;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    const sizeIndex = Math.max(0, Math.min(i, sizes.length - 1));
    return parseFloat((bytes / Math.pow(k, sizeIndex)).toFixed(dm)) + ' ' + sizes[sizeIndex];
  }

  // Export
  window.t = t;
  HostBerry.t = t;
  HostBerry.showAlert = showAlert;
  HostBerry.NOTIFICATION_AUTO_DISMISS_MS = HB_ALERT_AUTO_DISMISS_MS;
  HostBerry.attachTransientAlert = attachTransientAlert;
  HostBerry.dismissTransientAlert = dismissTransientAlert;
  HostBerry.scanVisibleAlerts = scanVisibleAlerts;
  HostBerry.apiRequest = apiRequest;
  HostBerry.getServerTimezone = getServerTimezone;
  HostBerry.formatTime = formatTime;
  HostBerry.formatUptime = formatUptime;
  HostBerry.formatBytes = formatBytes;
  HostBerry.escapeHtml = escapeHtml;

  // Verificar y mantener la sesión activa
  function setupSessionKeepAlive(){
    const page = document.body && document.body.getAttribute ? document.body.getAttribute('data-page') : '';
    const isAuthPage = (page === 'login' || page === 'first_login');
    if(isAuthPage) return;

    let consecutiveErrors = 0;
    const maxConsecutiveErrors = 3; // Permitir hasta 3 errores consecutivos antes de cerrar sesión
    
    // Verificar token periódicamente para detectar expiración antes de que ocurra
    // Verificar cada 2 minutos (si el token expira en 1 hora, esto es seguro)
    setInterval(async function(){
      try{
        // Hacer una petición simple para verificar si el token sigue válido (usa cookie HTTPOnly)
        const resp = await fetch('/api/v1/auth/me', { method: 'GET', credentials: 'include' });
        
        if(resp && resp.status === 401){
          // Verificar que realmente es un error de autenticación y no de red
          try {
            const errorData = await resp.json().catch(() => ({}));
            const errorMsg = String(errorData.error || '').toLowerCase();
            const errorCode = errorData.code || '';
            
            // Si es "Usuario no encontrado", puede ser un problema temporal - no cerrar sesión
            if(errorCode === 'USER_NOT_FOUND' || 
               errorMsg.includes('usuario no encontrado') ||
               errorMsg.includes('user not found')){
              console.warn('Usuario no encontrado durante keep-alive - puede ser problema temporal');
              // Resetear contador ya que no es un error de autenticación real
              consecutiveErrors = 0;
              return;
            }
            
            // Solo cerrar sesión si es un error real de token/autenticación
            if(errorMsg.includes('token') || errorMsg.includes('expirado') || 
               errorMsg.includes('expired') || errorMsg.includes('invalid') ||
               errorMsg.includes('autorizado') || errorMsg.includes('authorized')){
              consecutiveErrors++;
              if(consecutiveErrors >= maxConsecutiveErrors){
                console.warn('Token expirado después de múltiples intentos, redirigiendo a login...');
                window.location.href = '/login?error=session_expired';
              }
            } else {
              // Resetear contador si no es un error de autenticación real
              consecutiveErrors = 0;
            }
          } catch(_e) {
            // Si no se puede parsear, podría ser un error de red - no cerrar sesión
            consecutiveErrors = 0;
          }
        } else if(resp && resp.ok){
          // Token válido, resetear contador de errores
          consecutiveErrors = 0;
        }
      }catch(e){
        // Ignorar errores de red - no cerrar sesión por problemas de conectividad
        // Solo incrementar contador si es un error que no sea de red
        if(e.message && !e.message.includes('Failed to fetch') && 
           !e.message.includes('NetworkError') &&
           !e.message.includes('ERR_INTERNET_DISCONNECTED') &&
           !e.message.includes('ERR_NETWORK_CHANGED')){
          consecutiveErrors++;
          if(consecutiveErrors >= maxConsecutiveErrors){
            console.warn('Múltiples errores consecutivos, verificando sesión...');
            // Intentar una última verificación antes de cerrar
            consecutiveErrors = 0; // Resetear para dar otra oportunidad
          }
        } else {
          // Es un error de red, resetear contador
          consecutiveErrors = 0;
        }
      }
    }, 2 * 60 * 1000); // Cada 2 minutos (verificar más frecuentemente para sesión de 1 hora)
  }

  function setupSidebarToggle(){
    const sidebarToggle = document.querySelector('.sidebar-toggle');
    const sidebar = document.querySelector('.sidebar-nav');
    if(!sidebarToggle || !sidebar) return;

    sidebarToggle.addEventListener('click', function(e){
      e.preventDefault();
      e.stopPropagation();
      sidebar.classList.toggle('show');
    });

    // Cerrar sidebar al hacer clic fuera en móviles
    document.addEventListener('click', function(e){
      if(window.innerWidth > 991) return;
      if(!sidebar.classList.contains('show')) return;
      if(sidebar.contains(e.target) || sidebarToggle.contains(e.target)) return;
      sidebar.classList.remove('show');
    }, { passive: true });
  }

  document.addEventListener('DOMContentLoaded', async function(){
    // Limpiar token heredado en URL para reducir exposición en historial/referrer.
    try{
      const currentUrl = new URL(window.location.href);
      if(currentUrl.searchParams.has('token')){
        currentUrl.searchParams.delete('token');
        const nextUrl = currentUrl.pathname + (currentUrl.search ? currentUrl.search : '') + (currentUrl.hash || '');
        window.history.replaceState({}, '', nextUrl);
      }
    }catch(_e){
      // silent
    }

    setupSidebarToggle();
    setupSessionKeepAlive();

    // Aviso HTTP/HTTPS: solo mostrar si se está en HTTP (sin TLS)
    try{
      var isSecure = window.location.protocol === 'https:' ||
        (window.location.port === '' && window.location.protocol === 'https:');
      var warnEl = document.getElementById('hb-https-warning');
      if(warnEl){
        if(!isSecure){
          warnEl.style.display = 'block';
          warnEl.classList.remove('d-none');
          attachTransientAlert(warnEl);
        }else{
          dismissTransientAlert(warnEl);
        }
      }
    }catch(_e){}
    
    const el = document.getElementById('hb-current-username');
    const page = document.body && document.body.getAttribute ? document.body.getAttribute('data-page') : '';
    const isAuthPage = (page === 'login' || page === 'first_login');
    if(el && !isAuthPage){
      try{
        const resp = await apiRequest('/api/v1/auth/me');
        if(resp && resp.ok){
          const data = await resp.json();
          if(data && data.username){
            el.textContent = data.username;
          }
          var isAdmin = data && data.role && String(data.role).toLowerCase() === 'admin';
          if (typeof HostBerry !== 'undefined') HostBerry.isAdmin = isAdmin;

          if (!isAdmin) {
            document.body.classList.add('hb-readonly');
            document.querySelectorAll('[data-action="restart"],[data-action="shutdown"]').forEach(function(btn){
              btn.classList.add('d-none');
            });
            document.querySelectorAll('[data-requires-admin="true"]').forEach(function(btn){
              btn.classList.add('d-none');
            });
          } else {
            document.body.classList.remove('hb-readonly');
          }
        }
      }catch(_e){
        // silent
      }
    }

    scanVisibleAlerts();

    document.querySelectorAll('[data-action="logout"]').forEach(function(btn){
      btn.addEventListener('click', performLogout);
    });
    document.querySelectorAll('[data-action="logout-all"]').forEach(function(btn){
      btn.addEventListener('click', performLogoutAll);
    });
    
    document.querySelectorAll('[data-action="restart"]').forEach(function(btn){
      btn.addEventListener('click', performRestart);
    });
    
    document.querySelectorAll('[data-action="shutdown"]').forEach(function(btn){
      btn.addEventListener('click', performShutdown);
    });
  });

  // Compat: many views use showAlert() directly
  if(!window.showAlert){ window.showAlert = showAlert; }
  HostBerry.performLogout = performLogout;
  HostBerry.performLogoutAll = performLogoutAll;
  HostBerry.performRestart = performRestart;
  HostBerry.performShutdown = performShutdown;
  window.HostBerry = HostBerry;
})();
