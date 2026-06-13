#ifndef LIST_H
#define LIST_H

struct Node {
    int value;
    struct Node *next;
};

void list_push(struct Node **head, int value);
int list_length(struct Node *head);
void list_free(struct Node *head);
void list_print(struct Node *head);

#endif
