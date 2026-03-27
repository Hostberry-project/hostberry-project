// JS específico para login: toggles y alerts
(function(){
  // El aviso admin/admin se muestra solo cuando el backend indica credenciales por defecto (ShowDefaultCredentialsNotice).

  // i18n desde dataset HTML (multiidioma)
  const __i18nEl = document.getElementById('i18n-data');
  const i18n = {
    user_not_found: __i18nEl ? __i18nEl.getAttribute('data-user-not-found') : 'User not found',
    incorrect_password: __i18nEl ? __i18nEl.getAttribute('data-incorrect-password') : 'Incorrect password',
    too_many_attempts: __i18nEl ? __i18nEl.getAttribute('data-too-many-attempts') : 'Too many attempts',
    connection_error: __i18nEl ? __i18nEl.getAttribute('data-connection-error') : 'Connection error',
    login_generic_error: __i18nEl ? __i18nEl.getAttribute('data-login-generic-error') : 'Login error',
    login_success: __i18nEl ? __i18nEl.getAttribute('data-login-success') : 'Login successful'
  };

  // Toggle mostrar/ocultar contraseña (usa emoji de ojo)
  (function(){
    const btn = document.getElementById('toggle-password');
    if(!btn) return;
    btn.addEventListener('click', function(){
      const input = document.getElementById('password');
      const eye = document.getElementById('eye-emoji');
      const isPass = input.getAttribute('type') === 'password';
      input.setAttribute('type', isPass ? 'text' : 'password');
      if(eye) eye.textContent = isPass ? '🙈' : '👁️';
      const hideText = window.HostBerry && window.HostBerry.t ? window.HostBerry.t('common.hide_password', 'Hide password') : 'Hide password';
      const showText = window.HostBerry && window.HostBerry.t ? window.HostBerry.t('common.show_password', 'Show password') : 'Show password';
      this.setAttribute('aria-label', isPass ? hideText : showText);
      this.setAttribute('title', isPass ? hideText : showText);
    });
  })();

  // Alert helper: usar HostBerry.showAlert si existe, si no fallback local (p. ej. login carga antes)
  const showAlert = (type, message) => {
    if (window.HostBerry && typeof window.HostBerry.showAlert === 'function') {
      window.HostBerry.showAlert(type, message);
    } else {
      const alertDiv = document.createElement('div');
      alertDiv.className = `alert alert-${type} alert-dismissible fade show position-fixed d-flex align-items-center flex-nowrap hb-notification-row`;
      alertDiv.style.cssText = 'top:20px; left:50%; transform:translateX(-50%); z-index:9999; width:min(92vw,640px); max-width:92vw; margin:0;';
      alertDiv.setAttribute('role', 'alert');
      const msg = document.createElement('span');
      msg.className = 'flex-grow-1 hb-notification-message';
      msg.style.minWidth = '0';
      msg.textContent = String(message ?? '');
      const closeBtn = document.createElement('button');
      closeBtn.type = 'button';
      closeBtn.className = 'btn-close flex-shrink-0';
      closeBtn.setAttribute('aria-label', 'Close');
      let tmr = null;
      const dismiss = () => {
        if (tmr) clearTimeout(tmr);
        alertDiv.remove();
      };
      closeBtn.addEventListener('click', (e) => { e.preventDefault(); dismiss(); });
      alertDiv.appendChild(msg);
      alertDiv.appendChild(closeBtn);
      document.body.appendChild(alertDiv);
      tmr = setTimeout(dismiss, (window.HostBerry && window.HostBerry.NOTIFICATION_AUTO_DISMISS_MS) || 8000);
    }
  };

  // Manejador del login
  (function(){
    const form = document.getElementById('loginForm');
    if(!form) return;
    form.addEventListener('submit', async function(e){
      e.preventDefault();
      const fd = new FormData(this);
      const data = {
        username: fd.get('username'),
        password: fd.get('password')
      };
      try{
        const resp = await fetch('/api/v1/auth/login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          credentials: 'include',
          body: JSON.stringify(data)
        });
        const result = await resp.json().catch(function(){ return {}; });
        if(resp && resp.ok){
          // Sesión basada en cookie HTTPOnly (back activa "access_token" como HTTPOnly).
          showAlert('success', i18n.login_success);
          setTimeout(()=>{
            const lang = (document.documentElement && document.documentElement.lang === 'es') ? 'es' : 'en';
            const qs = `lang=${encodeURIComponent(lang)}`;
            if(result.password_change_required){
              window.location.href = `/first-login?${qs}`;
            } else {
              window.location.href = `/dashboard?${qs}`;
            }
          }, 800);
        } else {
          // Preferir mensaje real del backend si viene en {error: "..."}
          const backendError = (result && result.error) ? String(result.error) : '';
          const status = resp ? resp.status : 0;
          if(status === 422){
            showAlert('danger', backendError || i18n.login_generic_error);
            return;
          }
          if(status === 404){
            showAlert('warning', backendError || i18n.user_not_found);
          } else if(status === 401){
            // Puede ser contraseña incorrecta O ruta protegida por auth (token requerido)
            showAlert('danger', backendError || i18n.incorrect_password);
          } else if(status === 429){
            showAlert('warning', backendError || i18n.too_many_attempts);
          } else {
            showAlert('danger', backendError || i18n.login_generic_error);
          }
        }
      } catch(err){
        console.error(err);
        showAlert('danger', i18n.connection_error);
      }
    });
  })();
})();

