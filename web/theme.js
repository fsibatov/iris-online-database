(() => {
  try {
    const theme = localStorage.getItem('iris-theme');
    if (theme === 'light' || theme === 'dark') document.documentElement.dataset.theme = theme;
  } catch (_) {}
})();
