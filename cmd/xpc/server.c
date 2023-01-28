#include <stdio.h>
#include <stdlib.h>
#include <xpc/xpc.h>
#include <dispatch/dispatch.h>

int main(void)
{
    xpc_connection_t listener = xpc_connection_create_mach_service("com.apple.myservice", NULL, XPC_CONNECTION_MACH_SERVICE_LISTENER);
    xpc_connection_set_event_handler(listener, ^(xpc_object_t peer) {
      printf("New connection, peer=%p\n", peer);
      // It is safe to cast 'peer' to xpc_connection_t assuming
      // we have a correct configuration in our launchd.plist.
      xpc_connection_set_event_handler(peer, ^(xpc_object_t event) {
        // Handle event, whether it is a message or an error.
        if (event == XPC_ERROR_CONNECTION_INVALID) {
            printf("Connection closed by remote end\n");
            return;
        }

        if (xpc_get_type(event) != XPC_TYPE_DICTIONARY) {
            printf("Received something else than a dictionary!\n");
            return;
        }

        printf("Message received: %p\n", event);
		printf("%s\n", xpc_copy_description(event));

		xpc_object_t resp = xpc_dictionary_create(NULL, NULL, 0);
		xpc_dictionary_set_string(resp, "foo", "bar");
		xpc_connection_send_message(peer, resp);
      });
    xpc_connection_activate(peer);
    });
    xpc_connection_activate(listener);

    dispatch_main();
    exit(EXIT_FAILURE);
}
