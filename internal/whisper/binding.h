#ifndef WHISPER_BINDING_H
#define WHISPER_BINDING_H

#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

// Simple wrapper functions for whisper.cpp
void* whisper_load_model_from_file(const char *fname);
void whisper_free_model_wrapper(void* model);
int whisper_transcribe_wrapper(void* ctx, const float* audio_data, int audio_len, 
                              const char* language, char* result, int result_size,
                              float* segment_starts, float* segment_ends, char** segment_texts, int max_segments);
int whisper_get_supported_languages_wrapper(void* model, char** languages, int max_languages);
int whisper_is_multilingual_wrapper(void* model);

#ifdef __cplusplus
}
#endif

#endif // WHISPER_BINDING_H