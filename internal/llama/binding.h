#ifndef BINDING_H
#define BINDING_H

#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

// Model management
void* load_model(const char *fname, int n_ctx, int n_threads, int n_gpu_layers, bool use_mmap, bool use_mlock);
void* load_embedding_model(const char *fname, int n_ctx, int n_threads, int n_gpu_layers, bool use_mmap, bool use_mlock);
void free_model(void* model);

// Context management  
void* new_context(void* model, int n_ctx, int n_threads);
void* new_embedding_context(void* model, int n_ctx, int n_threads);
void free_context(void* ctx);
void clear_context(void* ctx);

// Text generation
int llama_predict(void* ctx, const char* prompt, char* result, int result_size,
                  int max_tokens, float temperature, float top_p, int top_k,
                  float repeat_penalty, int repeat_last_n, bool use_penalty);

// Grammar-constrained generation
int llama_predict_with_grammar(void* ctx, const char* prompt, char* result, int result_size,
                              int max_tokens, float temperature, float top_p, int top_k,
                              float repeat_penalty, int repeat_last_n, 
                              const char* grammar_str);

// Token utilities
int count_tokens(void* ctx, const char* text);
int get_context_size(void* model);

// GPU/Metal detection
bool has_gpu_support();

// Embedding generation
int llama_embedding(void* ctx, const char* text, float* embeddings, int max_embeddings);
int get_embedding_size(void* model);

// Model introspection
const char* get_model_architecture(void* model);
const char* get_model_name(void* model);
int get_model_parameter_count(void* model);
const char* get_model_quantization(void* model);
const char* get_model_family(void* model);
bool model_supports_images(void* model);
bool model_supports_audio(void* model);

#ifdef __cplusplus
}
#endif

#endif // BINDING_H