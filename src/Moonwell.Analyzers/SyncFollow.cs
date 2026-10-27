using System.Collections.Generic;
using System.Linq;
using System.Threading;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Diagnostics;
using Microsoft.CodeAnalysis.Operations;

namespace Moonwell.Analyzers;

/// <summary>
/// How the sync analyzer follows what code inside Local.For reaches: map methods and their overrides, the parts of a class
/// that run when it is made or first touched, operators, a using's Dispose, a foreach's enumerator, and the delegates
/// given to Local.For, to a parameter that ends there, or kept in a variable, field or event.
/// </summary>
public sealed partial class SyncAnalyzer
{
    /// <summary>Why a call may not run on one machine: the path from the call to what it reaches, and what that does.</summary>
    private sealed class Finding
    {
        public Finding(string path, string what, bool unseen = false)
        {
            Path = path;
            What = what;
            Unseen = unseen;
        }

        public string Path { get; }

        public string What { get; }

        /// <summary>Whether the code reached is code the analyzer cannot follow, which is MW0039 and not MW0001.</summary>
        public bool Unseen { get; }

        /// <summary>The name the path starts with: what the reported code calls.</summary>
        public string Name => Path.Split(new[] { " > " }, 2, System.StringSplitOptions.None)[0];

        public Finding Through(string name) => new(name + " > " + Path, What, Unseen);

        public string Message(string name) =>
            Path == name ? $"'{name}' {What}" : $"'{name}' {What} (through {Path})";
    }

    /// <summary>A member an operation calls or touches: whether it is written, the type of what it is called on, and whether the call is virtual.</summary>
    private readonly struct Reach
    {
        public Reach(ISymbol member, bool write = false, ITypeSymbol? receiver = null, bool isVirtual = false)
        {
            Member = member;
            Write = write;
            Receiver = receiver;
            IsVirtual = isVirtual;
        }

        public ISymbol? Member { get; }

        public bool Write { get; }

        public ITypeSymbol? Receiver { get; }

        public bool IsVirtual { get; }
    }

    /// <summary>The nodes being followed now, each with its depth, and the lowest depth a cycle was cut back to.</summary>
    private sealed class Walk
    {
        public Dictionary<object, int> Depths { get; } = new(new NodeComparer());

        public int Low { get; set; } = int.MaxValue;
    }

    /// <summary>A part of a class that is followed as one: its static initializers and constructor.</summary>
    private sealed class Part
    {
        public Part(INamedTypeSymbol type)
        {
            Type = type;
        }

        public INamedTypeSymbol Type { get; }

        public override bool Equals(object? other) => other is Part part && SymbolEqualityComparer.Default.Equals(part.Type, Type);

        public override int GetHashCode() => SymbolEqualityComparer.Default.GetHashCode(Type);
    }

    /// <summary>The values given to a delegate variable, field, property or event, followed as one.</summary>
    private sealed class Values
    {
        public Values(ISymbol slot)
        {
            Slot = slot;
        }

        public ISymbol Slot { get; }

        public override bool Equals(object? other) => other is Values values && SymbolEqualityComparer.Default.Equals(values.Slot, Slot);

        public override int GetHashCode() => SymbolEqualityComparer.Default.GetHashCode(Slot) + 1;
    }

    /// <summary>Compares followed nodes: symbols as Roslyn compares them, anything else by its own equality.</summary>
    private sealed class NodeComparer : IEqualityComparer<object>
    {
        public new bool Equals(object? x, object? y) =>
            x is ISymbol a && y is ISymbol b ? SymbolEqualityComparer.Default.Equals(a, b) : object.Equals(x, y);

        public int GetHashCode(object node) => node is ISymbol symbol ? SymbolEqualityComparer.Default.GetHashCode(symbol) : node.GetHashCode();
    }

    private sealed partial class Checker
    {
        private const string Unfollowed = "runs code the analyzer cannot follow";

        /// <summary>
        /// The members an operation calls or touches: the one it names, and those it calls without naming them (an
        /// operator, a user-defined conversion, a using's Dispose, a foreach's enumerator). A compound assignment or an
        /// increment of a property reads it and writes it.
        /// </summary>
        private IEnumerable<Reach> Targets(IOperation operation)
        {
            switch (operation)
            {
                case IInvocationOperation invocation:
                    yield return new Reach(invocation.TargetMethod, false, invocation.Instance?.Type, invocation.IsVirtual);
                    break;
                case IPropertyReferenceOperation property:
                    var virtualProperty = property.Instance is not null and not IInstanceReferenceOperation { Syntax: BaseExpressionSyntax };
                    yield return new Reach(property.Property, IsWritten(property), property.Instance?.Type, virtualProperty);
                    if (property.Parent is ICompoundAssignmentOperation or IIncrementOrDecrementOperation or ICoalesceAssignmentOperation)
                    {
                        yield return new Reach(property.Property, false, property.Instance?.Type, virtualProperty);
                    }

                    break;
                case IFieldReferenceOperation field:
                    yield return new Reach(field.Field, IsWritten(field));
                    break;
                case IMethodReferenceOperation reference:
                    yield return new Reach(reference.Method, false, reference.Instance?.Type, reference.IsVirtual);
                    break;
                case IObjectCreationOperation { Constructor: { } constructor }:
                    yield return new Reach(constructor);
                    break;
                case IBinaryOperation { OperatorMethod: { } binary }:
                    yield return new Reach(binary);
                    break;
                case IUnaryOperation { OperatorMethod: { } unary }:
                    yield return new Reach(unary);
                    break;
                case ICompoundAssignmentOperation { OperatorMethod: { } compound }:
                    yield return new Reach(compound);
                    break;
                case IIncrementOrDecrementOperation { OperatorMethod: { } increment }:
                    yield return new Reach(increment);
                    break;
                case IConversionOperation { OperatorMethod: { } conversion }:
                    yield return new Reach(conversion);
                    break;
                case IUsingOperation usingBlock:
                    foreach (var dispose in Disposes(usingBlock.Resources))
                    {
                        yield return dispose;
                    }

                    break;
                case IUsingDeclarationOperation usingDeclaration:
                    foreach (var dispose in Disposes(usingDeclaration.DeclarationGroup))
                    {
                        yield return dispose;
                    }

                    break;
                case IForEachLoopOperation { Syntax: CommonForEachStatementSyntax syntax } loop:
                    var info = loop.SemanticModel!.GetForEachStatementInfo(syntax);
                    var collection = Unwrap(loop.Collection).Type;
                    foreach (var method in new[] { info.GetEnumeratorMethod, info.MoveNextMethod, info.CurrentProperty?.GetMethod, info.DisposeMethod })
                    {
                        if (method != null)
                        {
                            yield return new Reach(method, false, SymbolEqualityComparer.Default.Equals(method, info.GetEnumeratorMethod) ? collection : null, true);
                        }
                    }

                    break;
            }
        }

        private static bool IsWritten(IOperation operation) => operation.Parent switch
        {
            IAssignmentOperation assignment => assignment.Target == operation,
            IIncrementOrDecrementOperation => true,
            _ => false,
        };

        /// <summary>The Dispose a using calls for each of its resources.</summary>
        private IEnumerable<Reach> Disposes(IOperation resources)
        {
            IEnumerable<ITypeSymbol?> types = resources is IVariableDeclarationGroupOperation group
                ? group.Declarations.SelectMany(d => d.Declarators).Select(d => (ITypeSymbol?)d.Symbol.Type)
                : new[] { resources.Type };
            var dispose = compilation.GetSpecialType(SpecialType.System_IDisposable).GetMembers("Dispose").OfType<IMethodSymbol>().FirstOrDefault();
            foreach (var type in types)
            {
                if (type == null || dispose == null)
                {
                    continue;
                }

                var method = type.TypeKind == TypeKind.Interface ? dispose : type.FindImplementationForInterfaceMember(dispose) as IMethodSymbol;
                if (method != null)
                {
                    yield return new Reach(method, false, type, true);
                }
            }
        }

        /// <summary>
        /// What an operation reaches that may not run on one machine: what its members do, the values of a delegate
        /// variable, field or event it runs, and a delegate it runs that the analyzer cannot follow.
        /// </summary>
        private Finding? Reaches(IOperation operation, Walk walk)
        {
            var from = EnclosingType(operation.Syntax, operation.SemanticModel!);
            foreach (var reach in Targets(operation))
            {
                if (Rejected(reach, walk, from) is { } found)
                {
                    return found;
                }
            }

            if (Slot(operation) is { } slot && FollowValues(slot, walk) is { } values)
            {
                return values.Through(Name(slot));
            }

            if (operation is IInvocationOperation { TargetMethod.MethodKind: MethodKind.DelegateInvoke, Instance: { } receiver } && !Followable(receiver))
            {
                return new Finding(Unwrap(receiver).Syntax.ToString(), Unfollowed, true);
            }

            return null;
        }

        /// <summary>
        /// Why a member may not be called on one machine; null when it may. The code that calls it is code of the class
        /// from (null for none), which is there already: touching its own members runs nothing of it for the first time.
        /// </summary>
        private Finding? Rejected(Reach reach, Walk walk, INamedTypeSymbol? from)
        {
            var member = reach.Member!;
            var write = reach.Write;
            var name = Name(member);
            if (IsNative(member))
            {
                if (member is IPropertySymbol)
                {
                    return write ? new Finding(name, "is set, which changes game state") : null;
                }

                if (Has(member, createsHandle))
                {
                    return new Finding(name, "creates a handle");
                }

                return Has(member, changesState) ? new Finding(name, "changes game state") : null;
            }

            if (member is IFieldSymbol or ILocalSymbol or IEventSymbol && !IsMap(member))
            {
                return null;
            }

            if ((runtime != null && SymbolEqualityComparer.Default.Equals(member.ContainingAssembly, runtime))
                || packages.Any(package => SymbolEqualityComparer.Default.Equals(member.ContainingAssembly, package)))
            {
                // Making a Moonwell object changes nothing in the game until it is added or started.
                if (member is IMethodSymbol { MethodKind: MethodKind.Constructor } || IsLocalSafe(member, write) || IsMachineLocal(member, write))
                {
                    return null;
                }

                return new Finding(name, "may change game state (it is not marked [LocalSafe])");
            }

            var methods = member switch
            {
                IMethodSymbol method => new[] { method },
                IPropertySymbol property => new[] { write ? property.SetMethod : property.GetMethod },
                _ => new IMethodSymbol?[0],
            };

            if (!IsMap(member))
            {
                // A library's virtual or interface member runs the map's overrides and implementations of it.
                foreach (var method in methods)
                {
                    foreach (var runs in method == null ? Enumerable.Empty<IMethodSymbol>() : Overriders(method, reach))
                    {
                        if (Follow(runs, walk) is { } inner)
                        {
                            return inner.Through(Name(runs)).Through(name);
                        }
                    }
                }

                return null;
            }

            // CSharp.lua runs a class's static initializers and static constructor when the class is first touched.
            if (member.ContainingType is { } type && !SymbolEqualityComparer.Default.Equals(type.OriginalDefinition, from?.OriginalDefinition)
                && FollowStatics(type.OriginalDefinition, walk) is { } statics)
            {
                return statics.Through("static " + type.Name + "()").Through(name);
            }

            if (member is IFieldSymbol or ILocalSymbol or IEventSymbol || Has(member, syncUnchecked) || IsMachineLocal(member, write))
            {
                return null;
            }

            if (member is IMethodSymbol { MethodKind: MethodKind.Constructor } constructor)
            {
                return FollowConstructor(constructor, walk)?.Through(name);
            }

            foreach (var method in methods)
            {
                if (method == null)
                {
                    continue;
                }

                if (Follow(method, walk) is { } inner)
                {
                    return inner.Through(name);
                }

                foreach (var runs in Overriders(method, reach))
                {
                    if (Follow(runs, walk) is { } overridden)
                    {
                        return overridden.Through(Name(runs)).Through(name);
                    }
                }
            }

            return null;
        }

        private bool IsMap(ISymbol member) => SymbolEqualityComparer.Default.Equals(member.ContainingAssembly, compilation.Assembly);

        /// <summary>
        /// The map's overrides and implementations of a virtual, abstract or interface method that a virtual call on a
        /// value of the receiver's type may run: those of a class the receiver's type converts to.
        /// </summary>
        private IEnumerable<IMethodSymbol> Overriders(IMethodSymbol method, Reach reach)
        {
            if (!reach.IsVirtual || !(method.IsVirtual || method.IsAbstract || method.IsOverride || method.ContainingType?.TypeKind == TypeKind.Interface))
            {
                yield break;
            }

            var seen = new HashSet<IMethodSymbol>(SymbolEqualityComparer.Default);
            var next = new Queue<IMethodSymbol>();
            next.Enqueue(method.OriginalDefinition);
            while (next.Count > 0)
            {
                var at = next.Dequeue();
                if (!overriders.Value.TryGetValue(at, out var below))
                {
                    continue;
                }

                foreach (var runs in below)
                {
                    if (!seen.Add(runs))
                    {
                        continue;
                    }

                    next.Enqueue(runs);
                    if (reach.Receiver == null || reach.Receiver.TypeKind == TypeKind.TypeParameter
                        || compilation.ClassifyCommonConversion(runs.ContainingType, reach.Receiver.OriginalDefinition).IsImplicit
                        || compilation.ClassifyCommonConversion(runs.ContainingType, reach.Receiver).IsImplicit)
                    {
                        yield return runs;
                    }
                }
            }
        }

        /// <summary>Each method or interface member of a class or interface, to the map's methods that override or implement it.</summary>
        private Dictionary<IMethodSymbol, List<IMethodSymbol>> FindOverriders()
        {
            var found = new Dictionary<IMethodSymbol, List<IMethodSymbol>>(SymbolEqualityComparer.Default);
            void Add(IMethodSymbol above, IMethodSymbol below)
            {
                above = above.OriginalDefinition;
                below = below.OriginalDefinition;
                if (SymbolEqualityComparer.Default.Equals(above, below))
                {
                    return;
                }

                if (!found.TryGetValue(above, out var list))
                {
                    found[above] = list = new List<IMethodSymbol>();
                }

                if (!list.Contains(below, SymbolEqualityComparer.Default))
                {
                    list.Add(below);
                }
            }

            foreach (var type in Types(compilation.Assembly.GlobalNamespace))
            {
                foreach (var method in type.GetMembers().OfType<IMethodSymbol>())
                {
                    if (method.OverriddenMethod is { } overridden)
                    {
                        Add(overridden, method);
                    }
                }

                if (type.TypeKind is not (TypeKind.Class or TypeKind.Struct))
                {
                    continue;
                }

                foreach (var face in type.AllInterfaces)
                {
                    foreach (var member in face.GetMembers().OfType<IMethodSymbol>())
                    {
                        if (type.FindImplementationForInterfaceMember(member) is IMethodSymbol implementation && IsMap(implementation))
                        {
                            Add(member, implementation);
                        }
                    }
                }
            }

            return found;
        }

        private static IEnumerable<INamedTypeSymbol> Types(INamespaceOrTypeSymbol container)
        {
            foreach (var member in container.GetMembers())
            {
                if (member is INamespaceSymbol space)
                {
                    foreach (var type in Types(space))
                    {
                        yield return type;
                    }
                }
                else if (member is INamedTypeSymbol type)
                {
                    yield return type;
                    foreach (var nested in Types(type))
                    {
                        yield return nested;
                    }
                }
            }
        }

        /// <summary>What a class's static field and property initializers and its static constructor reach.</summary>
        private Finding? FollowStatics(INamedTypeSymbol type, Walk walk) =>
            Follow(new Part(type), Initializers(type, true).Concat(type.StaticConstructors.SelectMany(Syntax)), walk);

        /// <summary>
        /// What making an object with a constructor of the map reaches: its body, its class's field and property
        /// initializers, and an implicit constructor's base constructor.
        /// </summary>
        private Finding? FollowConstructor(IMethodSymbol constructor, Walk walk)
        {
            constructor = constructor.OriginalDefinition;
            var type = constructor.ContainingType;
            var found = Follow(constructor, Syntax(constructor).Concat(Initializers(type, false)), walk);
            if (found == null && constructor.IsImplicitlyDeclared
                && type.BaseType?.InstanceConstructors.FirstOrDefault(c => c.Parameters.Length == 0) is { } baseConstructor)
            {
                found = Rejected(new Reach(baseConstructor), walk, type);
            }

            return found;
        }

        /// <summary>The initializers of a class's static or instance fields and properties.</summary>
        private static IEnumerable<SyntaxNode> Initializers(INamedTypeSymbol type, bool isStatic)
        {
            foreach (var member in type.GetMembers())
            {
                if (member.IsStatic != isStatic || member.IsImplicitlyDeclared || member is not (IFieldSymbol { IsConst: false } or IPropertySymbol))
                {
                    continue;
                }

                foreach (var reference in member.DeclaringSyntaxReferences)
                {
                    var initializer = reference.GetSyntax() switch
                    {
                        VariableDeclaratorSyntax variable => variable.Initializer,
                        PropertyDeclarationSyntax property => property.Initializer,
                        _ => null,
                    };
                    if (initializer != null)
                    {
                        yield return initializer;
                    }
                }
            }
        }

        // The declarations of a method; a primary constructor's is its class's, whose parameters run nothing.
        private static IEnumerable<SyntaxNode> Syntax(IMethodSymbol method) =>
            method.DeclaringSyntaxReferences.Select(reference => reference.GetSyntax()).Where(syntax => syntax is not TypeDeclarationSyntax);

        /// <summary>What a method of the map reaches that may not run on one machine.</summary>
        private Finding? Follow(IMethodSymbol method, Walk walk)
        {
            method = method.OriginalDefinition;
            return method.MethodKind == MethodKind.Constructor ? FollowConstructor(method, walk) : Follow(method, Syntax(method), walk);
        }

        /// <summary>
        /// What the code of a node (a method, a part of a class, the values of a delegate) reaches that may not run on
        /// one machine. A node met again while it is still being followed is a cycle, cut there with nothing found; a
        /// result is kept for later only when no cycle was cut back to a node that is still being followed above it,
        /// since that node's own code may still find something, so the answer never depends on what was followed first.
        /// Each value of a delegate must be code the analyzer can follow.
        /// </summary>
        private Finding? Follow(object node, IEnumerable<SyntaxNode> code, Walk walk)
        {
            if (followed.TryGetValue(node, out var known))
            {
                return known;
            }

            if (walk.Depths.TryGetValue(node, out var on))
            {
                walk.Low = System.Math.Min(walk.Low, on);
                return null;
            }

            var depth = walk.Depths.Count;
            walk.Depths[node] = depth;
            var outerLow = walk.Low;
            walk.Low = int.MaxValue;
            Finding? found = null;
            foreach (var syntax in code)
            {
                var body = compilation.GetSemanticModel(syntax.SyntaxTree).GetOperation(syntax);
                if (body == null)
                {
                    continue;
                }

                if (node is Values && !Followable(body))
                {
                    found = new Finding(syntax.ToString(), Unfollowed, true);
                    break;
                }

                foreach (var inner in body.DescendantsAndSelf())
                {
                    if ((found = Reaches(inner, walk)) != null)
                    {
                        break;
                    }
                }

                if (found != null)
                {
                    break;
                }
            }

            walk.Depths.Remove(node);
            var low = walk.Low;
            walk.Low = System.Math.Min(outerLow, low);
            if (found != null || low >= depth)
            {
                followed[node] = found;
            }

            return found;
        }

        // The class whose code a piece of syntax is.
        private static INamedTypeSymbol? EnclosingType(SyntaxNode syntax, SemanticModel model)
        {
            var symbol = model.GetEnclosingSymbol(syntax.SpanStart);
            return symbol as INamedTypeSymbol ?? symbol?.ContainingType;
        }

        // ---- delegates ----

        /// <summary>
        /// The delegate variable, field, property or event of the map an operation reads where the delegate may run:
        /// called, given as an argument, stored or returned. Null for any other operation; a parameter's delegates are
        /// the arguments of its callers, checked where they are written.
        /// </summary>
        private ISymbol? Slot(IOperation operation)
        {
            var slot = operation switch
            {
                ILocalReferenceOperation local => (ISymbol)local.Local,
                IFieldReferenceOperation field => field.Field,
                IPropertyReferenceOperation { Arguments.Length: 0 } property => property.Property,
                IEventReferenceOperation reference => reference.Event,
                _ => null,
            };
            if (slot == null || !IsMap(slot) || TypeOf(slot)?.TypeKind != TypeKind.Delegate)
            {
                return null;
            }

            var value = operation;
            var up = operation.Parent;
            while (up is IConversionOperation or IDelegateCreationOperation)
            {
                value = up;
                up = up.Parent;
            }

            var runs = up switch
            {
                IInvocationOperation invocation => invocation.Instance == value,
                IConditionalAccessOperation access => access.Operation == value,
                IArgumentOperation => true,
                ISimpleAssignmentOperation assignment => assignment.Value == value,
                ICompoundAssignmentOperation compound => compound.Value == value,
                IEventAssignmentOperation handler => handler.HandlerValue == value,
                IVariableInitializerOperation or IReturnOperation => true,
                _ => false,
            };
            return runs ? slot.OriginalDefinition : null;
        }

        private static ITypeSymbol? TypeOf(ISymbol symbol) => symbol switch
        {
            ILocalSymbol local => local.Type,
            IFieldSymbol field => field.Type,
            IPropertySymbol property => property.Type,
            IEventSymbol reference => reference.Type,
            IParameterSymbol parameter => parameter.Type,
            _ => null,
        };

        /// <summary>What the delegates given to a variable, field, property or event reach.</summary>
        private Finding? FollowValues(ISymbol slot, Walk walk) => Follow(new Values(slot), ValuesOf(slot), walk);

        /// <summary>The delegates given to a variable (in its body), or to a field, property or event (anywhere in the map): initializers and assignments.</summary>
        private IEnumerable<SyntaxNode> ValuesOf(ISymbol slot)
        {
            if (slot is ILocalSymbol local)
            {
                foreach (var reference in local.DeclaringSyntaxReferences)
                {
                    var declarator = reference.GetSyntax();
                    if (declarator is VariableDeclaratorSyntax { Initializer.Value: { } initial })
                    {
                        yield return initial;
                    }

                    var body = declarator.Ancestors().FirstOrDefault(a => a is BaseMethodDeclarationSyntax or AccessorDeclarationSyntax or LocalFunctionStatementSyntax or AnonymousFunctionExpressionSyntax or GlobalStatementSyntax);
                    if (body == null)
                    {
                        continue;
                    }

                    var model = compilation.GetSemanticModel(declarator.SyntaxTree);
                    foreach (var assignment in body.DescendantNodes().OfType<AssignmentExpressionSyntax>())
                    {
                        if (IsGiven(assignment) && SymbolEqualityComparer.Default.Equals(model.GetSymbolInfo(assignment.Left).Symbol, local))
                        {
                            yield return assignment.Right;
                        }
                    }
                }

                yield break;
            }

            if (memberValues.Value.TryGetValue(slot, out var values))
            {
                foreach (var value in values)
                {
                    yield return value;
                }
            }
        }

        private static bool IsGiven(AssignmentExpressionSyntax assignment) =>
            assignment.IsKind(SyntaxKind.SimpleAssignmentExpression) || assignment.IsKind(SyntaxKind.AddAssignmentExpression) || assignment.IsKind(SyntaxKind.CoalesceAssignmentExpression);

        /// <summary>Each delegate field, property and event of the map, to the delegates given to it: initializers and assignments.</summary>
        private Dictionary<ISymbol, List<SyntaxNode>> FindMemberValues()
        {
            var found = new Dictionary<ISymbol, List<SyntaxNode>>(SymbolEqualityComparer.Default);
            var names = new HashSet<string>();
            foreach (var type in Types(compilation.Assembly.GlobalNamespace))
            {
                foreach (var member in type.GetMembers())
                {
                    if (member.IsImplicitlyDeclared || member is not (IFieldSymbol or IPropertySymbol or IEventSymbol) || TypeOf(member)?.TypeKind != TypeKind.Delegate)
                    {
                        continue;
                    }

                    var values = found[member] = new List<SyntaxNode>();
                    names.Add(member.Name);
                    foreach (var reference in member.DeclaringSyntaxReferences)
                    {
                        var initial = reference.GetSyntax() switch
                        {
                            VariableDeclaratorSyntax variable => variable.Initializer?.Value,
                            PropertyDeclarationSyntax property => property.Initializer?.Value,
                            _ => null,
                        };
                        if (initial != null)
                        {
                            values.Add(initial);
                        }
                    }
                }
            }

            foreach (var tree in compilation.SyntaxTrees)
            {
                SemanticModel? model = null;
                foreach (var assignment in tree.GetRoot().DescendantNodes().OfType<AssignmentExpressionSyntax>())
                {
                    var named = assignment.Left switch
                    {
                        IdentifierNameSyntax identifier => identifier.Identifier.ValueText,
                        MemberAccessExpressionSyntax access => access.Name.Identifier.ValueText,
                        MemberBindingExpressionSyntax binding => binding.Name.Identifier.ValueText,
                        _ => null,
                    };
                    if (named == null || !names.Contains(named) || !IsGiven(assignment))
                    {
                        continue;
                    }

                    model ??= compilation.GetSemanticModel(tree);
                    if (model.GetSymbolInfo(assignment.Left).Symbol?.OriginalDefinition is { } member && found.TryGetValue(member, out var values))
                    {
                        values.Add(assignment.Right);
                    }
                }
            }

            return found;
        }

        /// <summary>
        /// Whether the code a delegate value runs can be followed: a lambda, a method, a parameter of a method (whose
        /// callers' arguments are checked), a variable, field, property or event of the map (whose values are followed),
        /// a map method's result (whose body is followed), or null.
        /// </summary>
        private bool Followable(IOperation value)
        {
            while (true)
            {
                switch (value)
                {
                    case IConversionOperation conversion:
                        value = conversion.Operand;
                        continue;
                    case IDelegateCreationOperation creation:
                        value = creation.Target;
                        continue;
                    case IParenthesizedOperation parenthesized:
                        value = parenthesized.Operand;
                        continue;
                    case IConditionalAccessInstanceOperation instance:
                        var access = instance.Parent;
                        while (access != null && access is not IConditionalAccessOperation)
                        {
                            access = access.Parent;
                        }

                        if (access is not IConditionalAccessOperation conditional)
                        {
                            return false;
                        }

                        value = conditional.Operation;
                        continue;
                    case IAnonymousFunctionOperation or IMethodReferenceOperation or ILocalReferenceOperation or IDefaultValueOperation:
                        return true;
                    case ILiteralOperation literal:
                        return literal.ConstantValue.HasValue && literal.ConstantValue.Value == null;
                    case IParameterReferenceOperation parameter:
                        return parameter.Parameter.ContainingSymbol is IMethodSymbol { MethodKind: not MethodKind.AnonymousFunction };
                    case IFieldReferenceOperation field:
                        return IsMap(field.Field);
                    case IPropertyReferenceOperation property:
                        return IsMap(property.Property);
                    case IEventReferenceOperation reference:
                        return IsMap(reference.Event);
                    case IInvocationOperation invocation:
                        return invocation.TargetMethod.MethodKind != MethodKind.DelegateInvoke && IsMap(invocation.TargetMethod);
                    case IConditionalOperation choice:
                        return Followable(choice.WhenTrue) && (choice.WhenFalse == null || Followable(choice.WhenFalse));
                    case ICoalesceOperation coalesce:
                        return Followable(coalesce.Value) && Followable(coalesce.WhenNull);
                    default:
                        return false;
                }
            }
        }

        // ---- where code runs on one machine ----

        /// <summary>Whether an operation runs on one machine: inside Local.For's action (or a delegate that ends there), or in a [MachineLocal] method.</summary>
        private bool IsLocal(IOperation operation, ISymbol container) =>
            IsLocal(operation.Syntax, operation.SemanticModel!, localParameters.Value, container);

        private bool IsLocal(SyntaxNode start, SemanticModel model, HashSet<IParameterSymbol> parameters, ISymbol? container)
        {
            for (var node = start; node != null; node = node.Parent)
            {
                if (node is ExpressionSyntax expression && InLocalPosition(expression, model, parameters))
                {
                    return true;
                }

                if (node is LocalFunctionStatementSyntax or AnonymousFunctionExpressionSyntax
                    && DeclaredBy(node, model) is { } function && Has(function, machineLocal))
                {
                    return true;
                }

                if (node is MemberDeclarationSyntax member)
                {
                    container ??= model.GetDeclaredSymbol(member);
                    break;
                }
            }

            return Has(container, machineLocal);
        }

        /// <summary>
        /// Whether an expression is a delegate that runs on one machine: Local.For's action, or an argument for a
        /// parameter that ends there.
        /// </summary>
        private bool InLocalPosition(ExpressionSyntax expression, SemanticModel model, HashSet<IParameterSymbol> parameters)
        {
            SyntaxNode node = expression;
            while (node.Parent is ParenthesizedExpressionSyntax or CastExpressionSyntax
                   || node.Parent is PostfixUnaryExpressionSyntax postfix && postfix.IsKind(SyntaxKind.SuppressNullableWarningExpression))
            {
                node = node.Parent;
            }

            if (node.Parent is not ArgumentSyntax argument || argument.Parent is not BaseArgumentListSyntax list || list.Parent == null)
            {
                return false;
            }

            if (model.GetSymbolInfo(list.Parent).Symbol is not IMethodSymbol method || ParameterOf(method, argument, list) is not { } parameter)
            {
                return false;
            }

            if (localFor != null && SymbolEqualityComparer.Default.Equals(method.OriginalDefinition, localFor))
            {
                return parameter.Ordinal == 1;
            }

            return parameters.Count > 0 && parameters.Contains(parameter.OriginalDefinition);
        }

        // The parameter an argument is for: by its name, else by its place (the last one when it is params).
        private static IParameterSymbol? ParameterOf(IMethodSymbol method, ArgumentSyntax argument, BaseArgumentListSyntax list)
        {
            if (argument.NameColon != null)
            {
                return method.Parameters.FirstOrDefault(p => p.Name == argument.NameColon.Name.Identifier.ValueText);
            }

            var index = list.Arguments.IndexOf(argument);
            if (index < method.Parameters.Length)
            {
                return method.Parameters[index];
            }

            return method.Parameters.Length > 0 && method.Parameters[method.Parameters.Length - 1].IsParams ? method.Parameters[method.Parameters.Length - 1] : null;
        }

        /// <summary>
        /// The delegate parameters of the map's methods and local functions whose delegate runs on one machine: one
        /// given to Local.For or to another such parameter, or called or passed on inside Local.For. Found by repeating
        /// until nothing changes, so the order of the code does not matter.
        /// </summary>
        private HashSet<IParameterSymbol> FindLocalParameters()
        {
            var uses = new Dictionary<IParameterSymbol, List<(IdentifierNameSyntax Name, SemanticModel Model)>>(SymbolEqualityComparer.Default);
            foreach (var tree in compilation.SyntaxTrees)
            {
                SemanticModel? model = null;
                foreach (var syntax in tree.GetRoot().DescendantNodes().OfType<ParameterSyntax>())
                {
                    if (syntax.Parent?.Parent is not (BaseMethodDeclarationSyntax or LocalFunctionStatementSyntax) || syntax.Type == null)
                    {
                        continue;
                    }

                    model ??= compilation.GetSemanticModel(tree);
                    if (model.GetDeclaredSymbol(syntax) is not IParameterSymbol { Type.TypeKind: TypeKind.Delegate } parameter)
                    {
                        continue;
                    }

                    var list = uses[parameter] = new List<(IdentifierNameSyntax, SemanticModel)>();
                    foreach (var name in syntax.Parent.Parent.DescendantNodes().OfType<IdentifierNameSyntax>())
                    {
                        if (name.Identifier.ValueText == parameter.Name && SymbolEqualityComparer.Default.Equals(model.GetSymbolInfo(name).Symbol, parameter))
                        {
                            list.Add((name, model));
                        }
                    }
                }
            }

            var local = new HashSet<IParameterSymbol>(SymbolEqualityComparer.Default);
            bool changed;
            do
            {
                changed = false;
                foreach (var pair in uses)
                {
                    if (local.Contains(pair.Key))
                    {
                        continue;
                    }

                    foreach (var (name, model) in pair.Value)
                    {
                        if (InLocalPosition(name, model, local) || ((IsCalled(name) || name.Parent is ArgumentSyntax) && IsLocal(name, model, local, null)))
                        {
                            local.Add(pair.Key);
                            changed = true;
                            break;
                        }
                    }
                }
            }
            while (changed);

            return local;
        }

        /// <summary>Whether a name of a delegate is called: a(), a.Invoke(), a?.Invoke().</summary>
        private static bool IsCalled(ExpressionSyntax name) => name.Parent switch
        {
            InvocationExpressionSyntax invocation => invocation.Expression == name,
            MemberAccessExpressionSyntax { Name.Identifier.ValueText: "Invoke" } access => access.Expression == name && access.Parent is InvocationExpressionSyntax,
            ConditionalAccessExpressionSyntax { WhenNotNull: InvocationExpressionSyntax { Expression: MemberBindingExpressionSyntax { Name.Identifier.ValueText: "Invoke" } } } access => access.Expression == name,
            _ => false,
        };

        /// <summary>
        /// MW0039 for a delegate given to Local.For, or to a parameter that ends there, that the analyzer cannot follow;
        /// MW0040 for such a parameter called where it runs on every machine.
        /// </summary>
        public void Delegates(OperationAnalysisContext context)
        {
            var operation = context.Operation;
            if (Silenced(operation, context.ContainingSymbol))
            {
                return;
            }

            if (operation is IArgumentOperation { Parameter: { } parameter } argument && parameter.Type.TypeKind == TypeKind.Delegate
                && (IsLocalForAction(parameter) || localParameters.Value.Contains(parameter.OriginalDefinition)) && !Followable(argument.Value))
            {
                var value = Unwrap(argument.Value).Syntax;
                context.ReportDiagnostic(Diagnostic.Create(Rules.Unfollowed, value.GetLocation(), new Finding(value.ToString(), Unfollowed, true).Message(value.ToString())));
            }

            if (operation is IParameterReferenceOperation reference && localParameters.Value.Contains(reference.Parameter.OriginalDefinition)
                && reference.Syntax is ExpressionSyntax name && IsCalled(name) && !IsLocal(operation, context.ContainingSymbol))
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.OneMachineCode, reference.Syntax.GetLocation(), reference.Parameter.Name,
                    "is given to Local.For, so the code given for it is checked as code of one machine, and here it runs on every machine; call it only inside Local.For"));
            }
        }

        private bool IsLocalForAction(IParameterSymbol parameter) =>
            localFor != null && parameter.Ordinal == 1 && SymbolEqualityComparer.Default.Equals(parameter.ContainingSymbol.OriginalDefinition, localFor);
    }
}
