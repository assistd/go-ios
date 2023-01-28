#include <stdio.h>
#include <stdlib.h>
#include <xpc/xpc.h>

static void
connection_handler(xpc_connection_t peer)
{
    xpc_connection_set_event_handler(peer, ^(xpc_object_t event) {
      printf("Message received: %p\n", event);
    });

    xpc_connection_resume(peer);
}

int main(int argc, char *argv[])
{
    xpc_connection_t conn;
    xpc_object_t msg;

    msg = xpc_dictionary_create(NULL, NULL, 0);
    xpc_dictionary_set_string(msg, "Hello", "world");

    if (argc < 2)
    {
        fprintf(stderr, "Usage: %s <mach service name>\n", argv[0]);
        return (1);
    }

    conn = xpc_connection_create_mach_service(argv[1], NULL, 0);
    if (conn == NULL)
    {
        perror("xpc_connection_create_mach_service");
        return (1);
    }

    xpc_connection_set_event_handler(conn, ^(xpc_object_t obj) {
      printf("[general] Received message in generic event handler: %p\n", obj);
      printf("[general] %s\n", xpc_copy_description(obj));
    });

    xpc_connection_resume(conn);
    xpc_connection_send_message(conn, msg);

    xpc_connection_send_message_with_reply(conn, msg, NULL, ^(xpc_object_t resp) {
      printf("[2]: Received second message: %p\n", resp);
      printf("[2]: %s\n", xpc_copy_description(resp));
    });

    xpc_connection_send_message_with_reply(conn, msg, NULL, ^(xpc_object_t resp) {
      printf("[3] Received third message: %p\n", resp);
      printf("[3] %s\n", xpc_copy_description(resp));
    });

    dispatch_main();
    exit(EXIT_FAILURE);
}
