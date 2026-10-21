using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Operations;
using Microsoft.CodeAnalysis.Text;

namespace Moonwell.Build
{
    /// <summary>
    /// Writes what is a float only at run time so that it reaches Lua as a float (C18). CSharp.lua drops a conversion from an
    /// integral type to <c>float</c> or <c>double</c> (<c>(float)count</c> and <c>float d = count * 1000000</c> give Lua the
    /// integer), so each gets <c>+ 0.0f</c> (<c>+ 0.0</c> for a double): the int is still worked out as an int first, as in C#,
    /// and the sum is a float in Lua. A conversion Lua gives a float or a bool anyway is left: an operand of <c>/</c>, of a
    /// comparison, of a compound assignment, or of <c>+ - * %</c> beside a float. CSharp.lua starts a <c>float</c> or
    /// <c>double</c> field, or auto-property, at the integer <c>0</c>, so one without an initializer gets <c>= 0.0f</c>.
    /// </summary>
    internal static class RunTimeFloats
    {
        /// <summary>Adds the edits of a file to <paramref name="edits"/>.</summary>
        internal static void Edits(SyntaxNode root, SemanticModel model, List<FloatEdit> edits)
        {
            foreach (var operation in Roots(root, model))
            {
                foreach (var conversion in operation.DescendantsAndSelf().OfType<IConversionOperation>())
                {
                    if (IsIntToFloat(conversion) && !LuaGivesFloat(conversion))
                    {
                        Converted(conversion, edits);
                    }
                }
            }

            foreach (var node in root.DescendantNodes())
            {
                switch (node)
                {
                    case VariableDeclaratorSyntax { Initializer: null, ArgumentList: null, Parent.Parent: FieldDeclarationSyntax field } declarator
                        when !field.Modifiers.Any(SyntaxKind.ConstKeyword) && Zero(model.GetDeclaredSymbol(declarator) as IFieldSymbol) is { } zero:
                        var name = declarator.Identifier;
                        edits.Add(new FloatEdit(new TextSpan(name.Span.End, 0), " = " + zero, 0, name.Text + " -> " + name.Text + " = " + zero));
                        break;
                    case PropertyDeclarationSyntax { Initializer: null, ExpressionBody: null, AccessorList: { } accessors } property
                        when IsAuto(property) && Zero(model.GetDeclaredSymbol(property)) is { } zero:
                        edits.Add(new FloatEdit(new TextSpan(accessors.Span.End, 0), " = " + zero + ";", 0, accessors + " -> " + accessors + " = " + zero));
                        break;
                }
            }
        }

        // The trees of operations of a file: member bodies and initializers.
        private static IEnumerable<IOperation> Roots(SyntaxNode node, SemanticModel model)
        {
            if (model.GetOperation(node) is { Parent: null } root)
            {
                yield return root;
                yield break;
            }

            foreach (var child in node.ChildNodes())
            {
                foreach (var operation in Roots(child, model))
                {
                    yield return operation;
                }
            }
        }

        // A conversion CSharp.lua drops from a value of an integral type (or an enum) to a float or double, known only at run time.
        private static bool IsIntToFloat(IOperation operation)
        {
            return operation is IConversionOperation { OperatorMethod: null, ConstantValue.HasValue: false } conversion
                && conversion.Type?.SpecialType is SpecialType.System_Single or SpecialType.System_Double
                && conversion.Operand.Type is { } type
                && (type.TypeKind == TypeKind.Enum || type.SpecialType is SpecialType.System_SByte or SpecialType.System_Byte
                    or SpecialType.System_Int16 or SpecialType.System_UInt16 or SpecialType.System_Int32 or SpecialType.System_UInt32
                    or SpecialType.System_Int64 or SpecialType.System_UInt64 or SpecialType.System_Char);
        }

        // Lua gives a float (or a bool) without the rewrite: / always gives a float, a comparison a bool, and + - * % a float beside
        // a float. Of two ints made floats, the left is rewritten, which makes the operation a float.
        private static bool LuaGivesFloat(IConversionOperation conversion)
        {
            switch (conversion.Parent)
            {
                case ICompoundAssignmentOperation:
                    return true;
                case IBinaryOperation binary:
                    switch (binary.OperatorKind)
                    {
                        case BinaryOperatorKind.Divide:
                        case BinaryOperatorKind.Equals:
                        case BinaryOperatorKind.NotEquals:
                        case BinaryOperatorKind.LessThan:
                        case BinaryOperatorKind.LessThanOrEqual:
                        case BinaryOperatorKind.GreaterThan:
                        case BinaryOperatorKind.GreaterThanOrEqual:
                            return true;
                        case BinaryOperatorKind.Add:
                        case BinaryOperatorKind.Subtract:
                        case BinaryOperatorKind.Multiply:
                        case BinaryOperatorKind.Remainder:
                            return binary.RightOperand == conversion || !IsIntToFloat(binary.RightOperand);
                        default:
                            return false;
                    }

                default:
                    return false;
            }
        }

        // (float)count becomes ((float)count + 0.0f); count, made a float without a cast, (count + 0.0f), with brackets around it
        // where + would bind it apart (count << 2).
        private static void Converted(IConversionOperation conversion, List<FloatEdit> edits)
        {
            ExpressionSyntax target;
            if (!conversion.IsImplicit)
            {
                if (conversion.Syntax is not CastExpressionSyntax cast)
                {
                    return;
                }

                target = cast;
            }
            else if (conversion.Operand.Syntax is ExpressionSyntax operand)
            {
                target = operand;
            }
            else
            {
                return;
            }

            var bracketed = !BindsBeforeAdd(target);
            var zero = conversion.Type!.SpecialType == SpecialType.System_Single ? "0.0f" : "0.0";
            var open = bracketed ? "((" : "(";
            var close = (bracketed ? ")" : string.Empty) + " + " + zero + ")";
            // At one place, the brackets that close come first, inner before outer, then a left-out argument (order 0), then
            // the brackets that open, outer before inner.
            var length = target.Span.Length;
            edits.Add(new FloatEdit(new TextSpan(target.SpanStart, 0), open, int.MinValue + length, string.Empty));
            edits.Add(new FloatEdit(new TextSpan(target.Span.End, 0), close, int.MaxValue - length, target + " -> " + open + target + close));
        }

        // An expression that stays whole before + 0.0f.
        private static bool BindsBeforeAdd(ExpressionSyntax expression)
        {
            switch (expression.Kind())
            {
                case SyntaxKind.IdentifierName:
                case SyntaxKind.GenericName:
                case SyntaxKind.SimpleMemberAccessExpression:
                case SyntaxKind.PointerMemberAccessExpression:
                case SyntaxKind.ConditionalAccessExpression:
                case SyntaxKind.InvocationExpression:
                case SyntaxKind.ElementAccessExpression:
                case SyntaxKind.ParenthesizedExpression:
                case SyntaxKind.NumericLiteralExpression:
                case SyntaxKind.CharacterLiteralExpression:
                case SyntaxKind.ThisExpression:
                case SyntaxKind.BaseExpression:
                case SyntaxKind.CastExpression:
                case SyntaxKind.CheckedExpression:
                case SyntaxKind.UncheckedExpression:
                case SyntaxKind.DefaultExpression:
                case SyntaxKind.SizeOfExpression:
                case SyntaxKind.PostIncrementExpression:
                case SyntaxKind.PostDecrementExpression:
                case SyntaxKind.PreIncrementExpression:
                case SyntaxKind.PreDecrementExpression:
                case SyntaxKind.UnaryMinusExpression:
                case SyntaxKind.UnaryPlusExpression:
                case SyntaxKind.BitwiseNotExpression:
                case SyntaxKind.AddExpression:
                case SyntaxKind.SubtractExpression:
                case SyntaxKind.MultiplyExpression:
                case SyntaxKind.DivideExpression:
                case SyntaxKind.ModuloExpression:
                    return true;
                default:
                    return false;
            }
        }

        // The zero a field or property of type float or double starts at, or null for any other, a struct's own (which C# lets
        // have no initializer without a constructor), or one in an interface.
        private static string? Zero(ISymbol? member)
        {
            var type = member switch
            {
                IFieldSymbol field => field.Type,
                IPropertySymbol property => property.Type,
                _ => null,
            };
            if (member == null || member.ContainingType.TypeKind == TypeKind.Interface || (member.ContainingType.TypeKind == TypeKind.Struct && !member.IsStatic))
            {
                return null;
            }

            return type?.SpecialType switch
            {
                SpecialType.System_Single => "0.0f",
                SpecialType.System_Double => "0.0",
                _ => null,
            };
        }

        // A property whose accessors have no body, which the compiler stores in a field of its own.
        private static bool IsAuto(PropertyDeclarationSyntax property)
        {
            return !property.Modifiers.Any(modifier => modifier.Kind() is SyntaxKind.AbstractKeyword or SyntaxKind.ExternKeyword or SyntaxKind.PartialKeyword)
                && property.AccessorList!.Accessors.All(accessor => accessor.Body == null && accessor.ExpressionBody == null);
        }
    }
}
