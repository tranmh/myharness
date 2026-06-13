#include <stdio.h>
#include "list.h"

int main(void) {
    struct Node *head = NULL;

    list_push(&head, 10);
    list_push(&head, 20);
    list_push(&head, 30);

    list_print(head);
    printf("Length: %d\n", list_length(head));

    list_free(head);
    return 0;
}
