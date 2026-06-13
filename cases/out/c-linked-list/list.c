#include <stdio.h>
#include <stdlib.h>
#include "list.h"

void list_push(struct Node **head, int value) {
    struct Node *node = malloc(sizeof(struct Node));
    node->value = value;
    node->next = *head;
    *head = node;
}

int list_length(struct Node *head) {
    int count = 0;
    while (head) {
        count++;
        head = head->next;
    }
    return count;
}

void list_free(struct Node *head) {
    while (head) {
        struct Node *next = head->next;
        free(head);
        head = next;
    }
}

void list_print(struct Node *head) {
    while (head) {
        printf("%d", head->value);
        if (head->next) printf(" ");
        head = head->next;
    }
    printf("\n");
}
