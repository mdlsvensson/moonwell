using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Operations;
using Microsoft.CodeAnalysis.Text;

namespace Moonwell.Build
{
    /// <summary>
    /// Writes <c>.ToString()</c> after a nullable value joined to a string (C21): CSharp.lua writes such a value, in a
    /// concatenation, after <c>+=</c> on a string, or in an interpolation hole with no width, as <c>n:ToString()</c>, a method of
    /// the Lua value, and a number or a boolean has methods only where CSharp.lua's core gives them through the debug library,
    /// which the game does not have; nil never has. <c>"n=" + n</c> becomes <c>"n=" + n.ToString()</c>, which CSharp.lua writes
    /// as <c>System.toString(n)</c>, and which C# defines as the same text: the value's, or <c>""</c> for null. A <c>char?</c>
    /// becomes <c>c?.ToString()</c>, which CSharp.lua writes with <c>System.Char.ToString</c>: a char is a number in Lua, which
    /// <c>System.toString</c> would give as its code. A hole with a format and no width, <c>{n:F1}</c>, becomes
    /// <c>{n?.ToString("F1")}</c> for an <c>IFormattable</c> (null gives <c>""</c> in the hole), or <c>{n.ToString()}</c> for
    /// any other type, whose format C# leaves out, and for a struct that implements <c>IFormattable</c> only explicitly, which
    /// CSharp.lua does not reach through the interface either (its <c>ToString()</c> is called). A nullable enum
    /// is left: CSharp.lua writes it with <c>System.EnumToString</c>, which gives <c>""</c> for nil. The brackets go outside
    /// those of <see cref="RunTimeFloats"/> around the same value (<c>"x" + (float?)count</c>).
    /// </summary>
    internal static class NullableText
    {
        /// <summary>Adds the edits of a file to <paramref name="edits"/>.</summary>
        internal static void Edits(SyntaxNode root, SemanticModel model, List<FloatEdit> edits)
        {
            // A primary constructor's operations may hold what its members' hold too: each value is rewritten once.
            var done = new HashSet<TextSpan>();
            foreach (var operation in RunTimeFloats.Roots(root, model))
            {
                foreach (var node in operation.DescendantsAndSelf())
                {
                    switch (node)
                    {
                        case IBinaryOperation { OperatorKind: BinaryOperatorKind.Add } binary when IsConcatenation(binary.Type, binary.OperatorMethod):
                            Joined(binary.LeftOperand, done, edits);
                            Joined(binary.RightOperand, done, edits);
                            break;
                        case ICompoundAssignmentOperation { OperatorKind: BinaryOperatorKind.Add } compound when IsConcatenation(compound.Type, compound.OperatorMethod):
                            Joined(compound.Value, done, edits);
                            break;
                        case IInterpolationOperation { Alignment: null } hole:
                            Interpolated(hole, done, edits);
                            break;
                    }
                }
            }
        }

        // A + that joins strings: one whose result is a string, with no operator of another type.
        private static bool IsConcatenation(ITypeSymbol? type, IMethodSymbol? method)
        {
            return type?.SpecialType == SpecialType.System_String && (method == null || method.ContainingType.SpecialType == SpecialType.System_String);
        }

        // n joined to a string becomes n.ToString(), c?.ToString() for a char.
        private static void Joined(IOperation operand, HashSet<TextSpan> done, List<FloatEdit> edits)
        {
            var value = Unconverted(operand);
            if (IsNullableValue(value.Type) && value.Syntax is ExpressionSyntax expression && done.Add(expression.Span))
            {
                Wrap(expression, Plain(value.Type!), edits);
            }
        }

        // {n} becomes {n.ToString()}; {n:F1} becomes {n?.ToString("F1")} for an IFormattable, {n.ToString()} for another type; a
        // char's {c} or {c:X} becomes {c?.ToString()}.
        private static void Interpolated(IInterpolationOperation hole, HashSet<TextSpan> done, List<FloatEdit> edits)
        {
            var value = Unconverted(hole.Expression);
            if (!IsNullableValue(value.Type) || value.Syntax is not ExpressionSyntax expression || hole.Syntax is not InterpolationSyntax syntax
                || !done.Add(expression.Span))
            {
                return;
            }

            if (syntax.FormatClause is not { } format)
            {
                Wrap(expression, Plain(value.Type!), edits);
                return;
            }

            var inner = Inner(value.Type!);
            var literal = SymbolDisplay.FormatLiteral(format.FormatStringToken.ValueText, quote: true);
            var arguments = inner.SpecialType != SpecialType.System_Char && IsFormattable(inner) ? FormattedToString(inner) : null;
            Wrap(expression, arguments == null ? Plain(value.Type!) : "?.ToString(" + literal + arguments + ")", edits);

            edits.Add(new FloatEdit(format.Span, string.Empty, 0, string.Empty));
        }

        // The call that gives a nullable value's text as C# joins it: n.ToString(), or c?.ToString() for a char, whose
        // Nullable<char>.ToString() CSharp.lua writes as System.toString, which gives the char's code.
        private static string Plain(ITypeSymbol type)
        {
            return Inner(type).SpecialType == SpecialType.System_Char ? "?.ToString()" : ".ToString()";
        }

        private static ITypeSymbol Inner(ITypeSymbol nullable)
        {
            return ((INamedTypeSymbol)nullable).TypeArguments[0];
        }

        // The value under the conversions C# makes implicitly (to object, for a concatenation).
        private static IOperation Unconverted(IOperation operation)
        {
            while (operation is IConversionOperation { IsImplicit: true } conversion)
            {
                operation = conversion.Operand;
            }

            return operation;
        }

        // A Nullable<T> of a value type other than an enum.
        private static bool IsNullableValue(ITypeSymbol? type)
        {
            return type is INamedTypeSymbol { OriginalDefinition.SpecialType: SpecialType.System_Nullable_T } nullable
                && nullable.TypeArguments[0].TypeKind != TypeKind.Enum;
        }

        // Whether C# passes a hole's format to the type: whether it implements IFormattable.
        private static bool IsFormattable(ITypeSymbol type)
        {
            return type.AllInterfaces.Any(face => IsSystem(face, "IFormattable"));
        }

        private static bool IsSystem(ITypeSymbol type, string name)
        {
            return type.Name == name && type.ContainingNamespace is { Name: "System", ContainingNamespace.IsGlobalNamespace: true };
        }

        // The arguments after the format of a public ToString the type has for it: "" for ToString(string), ", null" for
        // ToString(string, IFormatProvider); null when it has neither (IFormattable implemented explicitly).
        private static string? FormattedToString(ITypeSymbol type)
        {
            var methods = type.GetMembers("ToString").OfType<IMethodSymbol>()
                .Where(method => !method.IsStatic && method.DeclaredAccessibility == Accessibility.Public
                    && method.Parameters.Length > 0 && method.Parameters[0].Type.SpecialType == SpecialType.System_String)
                .ToList();
            if (methods.Any(method => method.Parameters.Length == 1))
            {
                return string.Empty;
            }

            return methods.Any(method => method.Parameters.Length == 2 && IsSystem(method.Parameters[1].Type, "IFormatProvider"))
                ? ", null"
                : null;
        }

        // The call after the expression, with brackets around it where the call would bind to a part of it ((n + 1), a?.Count);
        // outside the brackets RunTimeFloats writes around the same expression.
        private static void Wrap(ExpressionSyntax expression, string call, List<FloatEdit> edits)
        {
            var whole = IsWhole(expression);
            var open = whole ? string.Empty : "(";
            var close = (whole ? string.Empty : ")") + call;
            var length = expression.Span.Length;
            if (!whole)
            {
                edits.Add(new FloatEdit(new TextSpan(expression.SpanStart, 0), open, Brackets.Open(length, outer: true), string.Empty));
            }

            edits.Add(new FloatEdit(new TextSpan(expression.Span.End, 0), close, Brackets.Close(length, outer: true), expression + " -> " + open + expression + close));
        }

        // An expression a member access after it reads whole.
        private static bool IsWhole(ExpressionSyntax expression)
        {
            return expression.Kind() is SyntaxKind.IdentifierName or SyntaxKind.SimpleMemberAccessExpression or SyntaxKind.InvocationExpression
                or SyntaxKind.ElementAccessExpression or SyntaxKind.ParenthesizedExpression;
        }
    }
}
