import { defineConfig } from 'orval';

export default defineConfig({
    orvo: {
        output: {
            mode: 'tags-split',
            target: 'src/lib/api/endpoints',
            schemas: 'src/lib/api/model',
            client: 'svelte-query',
            baseUrl: '/api/v1',
            override: {
                namingConvention: {
                    enum: "kebab-case",
                },
            }
        },
        input: {
            target: '../openapi.yaml',
        },
    },
});