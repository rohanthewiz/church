- Remember the key changes above as they will be useful for this and other projects
- Principles for using element.Builder:
  1. Use element.NewBuilder() to create a builder, b.
  2. Use the builder methods to write a new element opening tag: b.DivClass(), b.H3(), etc.
  3. Use .R() for adding children and closing the current tag
  4. Or, Use .T() for text only content and closing the current tag
  5. Return b.String() at the end to retrieve the HTML string.