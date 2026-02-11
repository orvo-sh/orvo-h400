import { defineConfig } from 'orval';

export default defineConfig({
    orvo: {
        output: {
            mode: 'tags-split',
            target: 'src/lib/api/endpoints',
            schemas: 'src/lib/api/model',
            client: 'svelte-query',
            baseUrl: 'http://localhost:8080/api/v1',
            override: {
                namingConvention: {
                    enum: "kebab-case",
                },
                fetch: {
                    includeHttpStatusReturnType: false,
                },
                requestOptions: {
                    credentials: 'include',
                },
            }
        },
        input: {
            target: '../openapi.yaml',
        },
    },
});