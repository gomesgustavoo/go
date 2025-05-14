// cmd/ingest/ingest.go
// Servidor RTMP que recebe 1080p30 e decodifica via NVDEC usando cgo direto no Go

package main

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0
#include <gst/gst.h>
#include <gst/app/gstappsink.h>
#include <stdlib.h>

// Callback C para o sinal "new-sample"
// Removido 'static' para garantir que o símbolo seja exportado corretamente
GstFlowReturn on_new_sample(GstAppSink *appsink, gpointer user_data) {
    GstSample *sample = gst_app_sink_pull_sample(appsink);
    if (!sample) return GST_FLOW_ERROR;
    GstBuffer *buffer = gst_sample_get_buffer(sample);
    gsize size = gst_buffer_get_size(buffer);
    GstClockTime pts = GST_BUFFER_PTS(buffer);
    g_print("Frame recebido: %zu bytes, PTS: %lu\n", size, pts);
    gst_sample_unref(sample);
    return GST_FLOW_OK;
}
*/
import "C"
import (
	"os"
	"os/signal"
	"syscall"
	"unsafe"
)

func main() {
	// 1. Inicializa GStreamer (C API)
	C.gst_init(nil, nil)

	// 2. Descrição do pipeline: servidor RTMP + NVDEC
	desc := C.CString(
		`rtmpsrc location="listen://0.0.0.0:1935/live/stream" flvdemux name=demux demux.video_0 ! queue ! h264parse ! nvh264dec  cuda-memory=true ! video/x-raw(memory:NVMM),format=NV12 ! appsink name=sink emit-signals=true sync=false`,
	)
	defer C.free(unsafe.Pointer(desc))

	// 3. Cria o pipeline
	pipeline := C.gst_parse_launch(desc, nil)
	if pipeline == nil {
		os.Stderr.WriteString("Erro: não foi possível criar pipeline\n")
		os.Exit(1)
	}

	// 4. Obtém o appsink pelo nome e conecta callback
	cName := C.CString("sink")
	defer C.free(unsafe.Pointer(cName))
	sinkElem := C.gst_bin_get_by_name((*C.GstBin)(unsafe.Pointer(pipeline)), cName)
	if sinkElem == nil {
		os.Stderr.WriteString("Erro: appsink não encontrado\n")
		os.Exit(1)
	}
	// Configura o appsink para emitir sinais
	C.gst_app_sink_set_emit_signals((*C.GstAppSink)(unsafe.Pointer(sinkElem)), C.TRUE)
	// Conecta o callback C on_new_sample
	sigName := C.CString("new-sample")
	defer C.free(unsafe.Pointer(sigName))
	C.g_signal_connect_data(
		C.gpointer(unsafe.Pointer(sinkElem)),
		sigName,
		C.GCallback(C.on_new_sample),
		nil, nil, 0,
	)

	bus := C.gst_element_get_bus((*C.GstElement)(unsafe.Pointer(pipeline)))
	C.gst_bus_add_watch(bus, C.GstBusFunc(C.gst_bus_async_signal_func), nil)
	C.g_object_unref(C.gpointer(bus))

	// 5. Seta estado do pipeline para PLAYING
	C.gst_element_set_state((*C.GstElement)(unsafe.Pointer(pipeline)), C.GST_STATE_PLAYING)
	println("Servidor RTMP ativo: rtmp://0.0.0:1935/live/stream — Pressione Ctrl+C para parar.")

	// 6. Aguarda sinal de interrupção
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	loop := C.g_main_loop_new(nil, 0)
	go func() { <-sig; C.g_main_loop_quit(loop) }()
	C.g_main_loop_run(loop)

	// 7. Finaliza o pipeline
	C.gst_element_set_state((*C.GstElement)(unsafe.Pointer(pipeline)), C.GST_STATE_NULL)
	C.g_main_loop_unref(loop)
	println("Pipeline finalizado. Servidor parado.")
}
