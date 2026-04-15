import axios from 'axios';

const apiClient = axios.create({
  baseURL: window.location.origin,
  headers: { 'Content-Type': 'application/json' },
  withCredentials: true,
});

apiClient.interceptors.response.use(
  (response) => {
    if (response.data?.code === 10086) {
      window.dispatchEvent(new CustomEvent('auth-expired'));
    }
    return response;
  },
  (error) => {
    return Promise.reject(error);
  },
);

export default apiClient;
