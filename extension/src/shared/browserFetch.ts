// A bare fetch stored on an object is called with this bound to the holder, which browsers reject
// with "Illegal invocation". Every stored fetch default goes through this wrapper.
export const browserFetch: typeof fetch = (input, init) => fetch(input, init);
