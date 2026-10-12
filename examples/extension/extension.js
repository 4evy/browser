const { name, store, version, records } = input.files.database;
const database = await new Promise((resolve, reject) => {
  const request = indexedDB.open(name, version);
  request.onupgradeneeded = () => {
    if (!request.result.objectStoreNames.contains(store)) {
      request.result.createObjectStore(store, { keyPath: "id" });
    }
  };
  request.onsuccess = () => resolve(request.result);
  request.onerror = () => reject(request.error);
  request.onblocked = () => reject(new Error("Database upgrade is blocked"));
});

try {
  await new Promise((resolve, reject) => {
    const transaction = database.transaction(store, "readwrite");
    transaction.oncomplete = resolve;
    transaction.onabort = () => reject(transaction.error);
    for (const record of records) transaction.objectStore(store).put(record);
  });
} finally {
  database.close();
}
