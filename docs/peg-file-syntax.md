# PEG file syntax

## Examples of PEG grammars

Here are some projects that use `peg` to provide further examples of PEG grammars:

* https://github.com/tj/go-naturaldate - natural date/time parsing
* https://github.com/gnames/gnparser - scientific names parsing

## Go package and imports

First declare the package name and any import(s) required.

```
package <package name>

import "<import path>"
import <alias> "<import path>"
```

## Parser

Then declare the parser:

```
type <parser name> Peg {
	<parser state variables>
}
```

## Rules

Next declare the rules. Note that the main rules are described below but are based on the [peg/leg rules](https://www.piumarta.com/software/peg/peg.1.html) which provide additional documentation.

The first rule is the entry point into the parser:

```
<rule name> <- <rule body>
```

The first rule should probably end with `!.` to indicate no more input follows:

```
first <- . !.
```

This is often set to `END` to make PEG rules more readable:

```
END <- !.
```

`.` means any character matches. For zero or more character matches, use:

```
repetition <- .*
```

For one or more character matches, use:

```
oneOrMore <- .+
```

For an optional character match, use:

```
optional <- .?
```

If specific characters are to be matched, use single quotes:

```
specific <- 'a'* 'bc'+ 'de'?
```

This will match the string `"aaabcbcde"`.

For choosing between different inputs, use alternates:

```
prioritized <- 'a' 'a'* / 'bc'+ / 'de'?
```

This will match `"aaaa"` or `"bcbc"` or `"de"` or `""`. The matches are attempted in order.

If the characters are case-insensitive, use double quotes:

```
insensitive <- "abc"
```

This will match `"abc"` or `"Abc"` or `"ABc"` and so on.

For matching a set of characters, use a character class:

```
class <- [a-z]
```

This will match `"a"` or `"b"` or all the way to `"z"`.

For an inverse character class, start with a caret:

```
inverse <- [^a-z]
```

This will match anything but `"a"` or `"b"` or all the way to `"z"`.

If the character class is case-insensitive, use double brackets:

```
insensitive <- [[A-Z]]
```

(Note that this is not available in regular expression syntax.)

Use parentheses for grouping:

```
grouping <- (rule1 / rule2) rule3
```

For looking ahead a match (predicate), use:

```
lookAhead <- &rule1 rule2
```

For inverse look ahead, use:

```
inverse <- !rule1 rule2
```

Use curly braces for Go code:

```
gocode <- { fmt.Println("hello world") }
```

For string captures, use less than and greater than:

```
capture <- <'capture'> { fmt.Println(text) }
```

Will print out `"capture"`. The captured string is stored in `buffer[begin:end]`.

## Left recursion

By default a rule that calls itself at the same input position (for example
`Expr <- Expr '-' Term / Term`) is reported as a possible infinite left
recursion, because a plain recursive-descent parser would enter it forever.

Generate with the `-leftrecursion` flag to support such rules directly:

```
go tool peg -leftrecursion grammar.peg
```

The existing `.peg` syntax is unchanged; the flag only changes the generated
parser. Both direct and mutual left recursion are supported:

```
Expr <- Expr '-' Term / Term
Term <- < [0-9]+ >
```

```
Expr <- Stmt / Num
Stmt <- Expr '-' Num
```

Continuous call and member suffixes can be written the same way:

```
Post <- Post '(' ')' / Post '.' Name / Name
```

The parser first looks for a base derivation (the seed), then re-evaluates the
rule while the candidate keeps consuming more input. Candidates ending at the
same position keep the declared alternative order. A recursive cycle without
a successful base fails with an ordinary parse error instead of looping
forever, for example `Loop <- Loop`.

The generated syntax tree keeps the original named rule nesting, so `8-3-1`
is reported as `Expr(8-3-1)` containing `Expr(8-3)` containing `Expr(8)`,
not as a flattened repetition. Captures and deferred Go actions belong to the
single winning derivation only: losing alternatives, attempts that did not
grow, and shorter results leave no extra tokens or actions. As always,
`Parse` itself does not run Go actions; one subsequent `Execute` runs the
action stream of the winning derivation in order.

Notes and limitations:

* Left recursion mode requires the AST, so it cannot be combined with
  `-noast`; the combination is reported as an error. The `DisableMemoize`
  parser option is also rejected by `Parse`.
* Semantic predicates that depend on the result of a recursive call are not
  specially tracked. Predicates outside the recursion, such as the usual
  trailing `!.`, work normally.
* `-inline` and `-switch` remain usable: inlining is conservatively disabled
  for the recursive rules themselves, so the public parse result, syntax tree
  and action stream are unchanged.
* Grammars without left recursive rules generate exactly the same code with
  or without the flag, and the flag is off by default.
