export function showToast(msg: string, isError: boolean = false) {
  const toast = document.getElementById('toast');
  if (!toast) return;
  toast.innerText = msg;
  toast.className = 'toast' + (isError ? ' error' : '') + ' show';
  setTimeout(() => {
    toast.className = 'toast';
  }, 3500);
}
