using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Diagnostics;
using Microsoft.CodeAnalysis.Operations;

namespace Moonwell.Analyzers;

/// <summary>Rejects the multiplayer mistakes of decision 13 in a map project.</summary>
[DiagnosticAnalyzer(LanguageNames.CSharp)]
public sealed partial class SyncAnalyzer : DiagnosticAnalyzer
{
    public override ImmutableArray<DiagnosticDescriptor> SupportedDiagnostics => Rules.All;

    public override void Initialize(AnalysisContext context)
    {
        context.ConfigureGeneratedCodeAnalysis(GeneratedCodeAnalysisFlags.None);
        context.EnableConcurrentExecution();
        context.RegisterCompilationStartAction(start =>
        {
            var checker = new Checker(start.Compilation);
            start.RegisterOperationAction(checker.Call,
                OperationKind.Invocation, OperationKind.PropertyReference, OperationKind.MethodReference, OperationKind.ObjectCreation, OperationKind.FieldReference);
            start.RegisterOperationAction(checker.Loop, OperationKind.Loop);
            start.RegisterOperationAction(checker.Conversion, OperationKind.Conversion);
            start.RegisterOperationAction(checker.Literal, OperationKind.Literal);
            start.RegisterOperationAction(checker.Declarator, OperationKind.VariableDeclarator);
            start.RegisterOperationAction(checker.Text, OperationKind.Binary, OperationKind.Interpolation, OperationKind.Conversion);
            start.RegisterOperationAction(checker.Implicit,
                OperationKind.Binary, OperationKind.Unary, OperationKind.CompoundAssignment, OperationKind.Increment, OperationKind.Decrement,
                OperationKind.Conversion, OperationKind.Using, OperationKind.UsingDeclaration, OperationKind.Loop, OperationKind.LocalReference,
                OperationKind.EventReference);
            start.RegisterOperationAction(checker.Delegates, OperationKind.Argument, OperationKind.ParameterReference);
            start.RegisterOperationAction(checker.HashArgument, OperationKind.Argument);
            start.RegisterOperationAction(checker.LambdaPlacement, OperationKind.AnonymousFunction);
            start.RegisterSymbolAction(checker.Placement, SymbolKind.Method, SymbolKind.Property);
            start.RegisterSymbolAction(checker.Declaration, SymbolKind.Field, SymbolKind.Property, SymbolKind.Method, SymbolKind.NamedType);
        });
    }

    /// <summary>The checks of one compilation, with the symbols they compare against.</summary>
    private sealed partial class Checker
    {
        private readonly Compilation compilation;
        private readonly INamedTypeSymbol? natives;
        private readonly IMethodSymbol? localFor;
        private readonly IAssemblySymbol? runtime;
        /// <summary>Moonwell's packages that the map references, checked as the runtime is.</summary>
        private readonly IAssemblySymbol[] packages;
        private readonly INamedTypeSymbol? createsHandle;
        private readonly INamedTypeSymbol? changesState;
        private readonly INamedTypeSymbol? machineLocal;
        private readonly INamedTypeSymbol? desyncs;
        private readonly INamedTypeSymbol? localSafe;
        private readonly INamedTypeSymbol? syncUnchecked;
        private readonly INamedTypeSymbol?[] hashTypes;
        /// <summary>The interfaces of the collections that give their items in no fixed order.</summary>
        private readonly INamedTypeSymbol?[] hashInterfaces;
        /// <summary>The interfaces that hand out a collection's items: what a loop or LINQ goes through.</summary>
        private readonly INamedTypeSymbol?[] collections;
        private readonly INamedTypeSymbol? handle;

        /// <summary>What each followed node of the map reaches; a node that reaches nothing rejected maps to null.</summary>
        private readonly ConcurrentDictionary<object, Finding?> followed = new(new NodeComparer());

        /// <summary>The map's overrides and implementations of each method, found once.</summary>
        private readonly Lazy<Dictionary<IMethodSymbol, List<IMethodSymbol>>> overriders;

        /// <summary>The delegates given to each delegate field, property and event of the map, found once.</summary>
        private readonly Lazy<Dictionary<ISymbol, List<SyntaxNode>>> memberValues;

        /// <summary>The delegate parameters whose delegate runs on one machine, found once.</summary>
        private readonly Lazy<HashSet<IParameterSymbol>> localParameters;

        public Checker(Compilation compilation)
        {
            this.compilation = compilation;
            natives = compilation.GetTypeByMetadataName("Moonwell.Natives");
            var local = compilation.GetTypeByMetadataName("Moonwell.Local");
            localFor = local?.GetMembers("For").OfType<IMethodSymbol>().FirstOrDefault();
            runtime = local?.ContainingAssembly;
            // Each package is found by one of its types.
            packages = new[] { "Moonwell.DamageSystem", "Moonwell.Vector2", "Moonwell.Find", "Moonwell.Terrain", "Moonwell.Area", "Moonwell.Mouse" }
                .Select(name => compilation.GetTypeByMetadataName(name)?.ContainingAssembly)
                .Where(assembly => assembly != null)
                .Select(assembly => assembly!)
                .ToArray();
            createsHandle = compilation.GetTypeByMetadataName("Moonwell.CreatesHandleAttribute");
            changesState = compilation.GetTypeByMetadataName("Moonwell.ChangesStateAttribute");
            machineLocal = compilation.GetTypeByMetadataName("Moonwell.MachineLocalAttribute");
            desyncs = compilation.GetTypeByMetadataName("Moonwell.DesyncsAttribute");
            localSafe = compilation.GetTypeByMetadataName("Moonwell.LocalSafeAttribute");
            syncUnchecked = compilation.GetTypeByMetadataName("Moonwell.SyncUncheckedAttribute");
            hashTypes = new[]
            {
                compilation.GetTypeByMetadataName("System.Collections.Generic.Dictionary`2"),
                compilation.GetTypeByMetadataName("System.Collections.Generic.Dictionary`2+KeyCollection"),
                compilation.GetTypeByMetadataName("System.Collections.Generic.Dictionary`2+ValueCollection"),
                compilation.GetTypeByMetadataName("System.Collections.Generic.HashSet`1"),
            };
            hashInterfaces = new[]
            {
                compilation.GetTypeByMetadataName("System.Collections.Generic.IDictionary`2"),
                compilation.GetTypeByMetadataName("System.Collections.Generic.IReadOnlyDictionary`2"),
                compilation.GetTypeByMetadataName("System.Collections.Generic.ISet`1"),
                compilation.GetTypeByMetadataName("System.Collections.Generic.IReadOnlySet`1"),
            };
            collections = new[]
            {
                compilation.GetTypeByMetadataName("System.Collections.IEnumerable"),
                compilation.GetTypeByMetadataName("System.Collections.Generic.IEnumerable`1"),
                compilation.GetTypeByMetadataName("System.Collections.ICollection"),
                compilation.GetTypeByMetadataName("System.Collections.Generic.ICollection`1"),
                compilation.GetTypeByMetadataName("System.Collections.Generic.IReadOnlyCollection`1"),
            };
            handle = compilation.GetTypeByMetadataName("Moonwell.handle");
            overriders = new(FindOverriders, LazyThreadSafetyMode.ExecutionAndPublication);
            memberValues = new(FindMemberValues, LazyThreadSafetyMode.ExecutionAndPublication);
            localParameters = new(FindLocalParameters, LazyThreadSafetyMode.ExecutionAndPublication);
        }

        public void Call(OperationAnalysisContext context)
        {
            var operation = context.Operation;
            var reach = Targets(operation).FirstOrDefault();
            var member = reach.Member;
            var write = reach.Write;
            if (member == null || Silenced(operation, context.ContainingSymbol))
            {
                return;
            }

            var location = operation.Syntax.GetLocation();
            if (operation is IObjectCreationOperation { Constructor: { Parameters.Length: 0 } constructor } && IsNamed(constructor.ContainingType, "System.Random"))
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.UnseededRandom, location, Name(constructor)));
                return;
            }

            if (IsNative(member) && member.Name is "GetHandleId" or "GetHandleIdBJ")
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.HandleId, location));
            }

            if (member.Name is "GetEnumerator" or "CopyTo"
                && (IsHash(member.ContainingType) ? member.ContainingType.Name : operation is IInvocationOperation { Instance: { } instance } ? HashName(instance) : null) is { } hash)
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.HashLoop, location, Article(hash)));
            }

            if (Has(member, desyncs))
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.Desyncs, location, Name(member)));
            }
            else if (IsLocal(operation, context.ContainingSymbol))
            {
                ReportLocal(context, operation);
            }
            else if (IsMachineLocal(member, write) && !DeclaredName(operation, member, write) && !InNameOf(operation))
            {
                // 0.2.2's Name read the declared name outside the constructor too; the message says where it is now.
                var declared = member is IPropertySymbol { Name: "Name" } && IsRuntime(member) && member.ContainingType.Name == "Unit" ? "; the declared name is Default.Name" : string.Empty;
                context.ReportDiagnostic(Diagnostic.Create(Rules.LocalOutside, location, Name(member), declared));
            }
            else if (LibraryLocal(operation, member) is { } why)
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.LocalOutside, location, Name(member), why));
            }
        }

        /// <summary>MW0001 (or MW0039) for what Local.For reaches without a name: an operator, a conversion, a using's Dispose, a foreach's enumerator, and a delegate variable or event it runs.</summary>
        public void Implicit(OperationAnalysisContext context)
        {
            var operation = context.Operation;
            var named = operation switch
            {
                IBinaryOperation binary => binary.OperatorMethod != null,
                IUnaryOperation unary => unary.OperatorMethod != null,
                ICompoundAssignmentOperation compound => compound.OperatorMethod != null,
                IIncrementOrDecrementOperation increment => increment.OperatorMethod != null,
                IConversionOperation conversion => conversion.OperatorMethod != null,
                ILocalReferenceOperation local => local.Local.Type.TypeKind == TypeKind.Delegate,
                IEventReferenceOperation => true,
                IUsingOperation or IUsingDeclarationOperation or IForEachLoopOperation => true,
                _ => false,
            };
            if (named && !Silenced(operation, context.ContainingSymbol) && IsLocal(operation, context.ContainingSymbol))
            {
                ReportLocal(context, operation);
            }
        }

        /// <summary>Reports what an operation inside Local.For reaches: MW0001, or MW0039 for code the analyzer cannot follow.</summary>
        private void ReportLocal(OperationAnalysisContext context, IOperation operation)
        {
            if (Reaches(operation, new Walk()) is not { } finding)
            {
                return;
            }

            var location = operation is IForEachLoopOperation loop ? loop.Collection.Syntax.GetLocation()
                : operation is IUsingOperation usingBlock ? usingBlock.Resources.Syntax.GetLocation()
                : operation.Syntax.GetLocation();
            context.ReportDiagnostic(Diagnostic.Create(finding.Unseen ? Rules.Unfollowed : Rules.StateInLocal, location, finding.Message(finding.Name)));
        }

        private const string Clock = "; it is this machine's clock: time game logic with Timer.After or Timer.Every";

        private const string Address = "; in the game's Lua it is the object's address: key by the object itself, or by a number the map assigns";

        private const string HandleText = "; in the game's Lua a handle as text is its address";

        /// <summary>
        /// Why a .NET library member gives a different answer on each machine, as the suffix of MW0002's message: the
        /// clock, or an address in the game's Lua (the hash of a handle or of a class without its own, a handle as text).
        /// Null for any other member.
        /// </summary>
        private string? LibraryLocal(IOperation operation, ISymbol member)
        {
            var type = member.ContainingType;
            if (type == null)
            {
                return null;
            }

            if (member is IPropertySymbol { Name: "Now" or "UtcNow" or "Today" } && (IsNamed(type, "System.DateTime") || IsNamed(type, "System.DateTimeOffset")))
            {
                return Clock;
            }

            if (member is IPropertySymbol { Name: "TickCount" or "TickCount64" } && IsNamed(type, "System.Environment"))
            {
                return Clock;
            }

            if (member.Name is "Elapsed" or "ElapsedMilliseconds" or "ElapsedTicks" or "GetTimestamp" or "GetElapsedTime" && IsNamed(type, "System.Diagnostics.Stopwatch"))
            {
                return Clock;
            }

            if (member is IMethodSymbol { Name: "GetHashCode", IsStatic: true } && IsNamed(type, "System.Runtime.CompilerServices.RuntimeHelpers"))
            {
                return Address;
            }

            if (operation is not IInvocationOperation { Instance.Type: { } receiver, TargetMethod: { IsStatic: false, Parameters.Length: 0 } })
            {
                return null;
            }

            if (member.Name == "GetHashCode" && HashIsAddress(receiver))
            {
                return Address;
            }

            return member.Name == "ToString" && IsHandle(receiver) ? HandleText : null;
        }

        /// <summary>
        /// Whether GetHashCode of a value of a type is its address in the game's Lua: a handle, or a class with no
        /// GetHashCode of its own below object (a record, a string, an anonymous type and a struct hash their content).
        /// </summary>
        private bool HashIsAddress(ITypeSymbol type)
        {
            if (IsHandle(type))
            {
                return true;
            }

            if (type is not INamedTypeSymbol { TypeKind: TypeKind.Class, IsAnonymousType: false } named || named.SpecialType != SpecialType.None)
            {
                return false;
            }

            for (var at = named; at != null && at.SpecialType != SpecialType.System_Object; at = at.BaseType)
            {
                if (at.GetMembers("GetHashCode").OfType<IMethodSymbol>().Any(m => m.IsOverride && m.Parameters.Length == 0))
                {
                    return false;
                }
            }

            return true;
        }

        /// <summary>MW0002 for a handle turned into text outside Local.For: joined to a string, in an interpolation hole, or given as an object to a library method that writes text.</summary>
        public void Text(OperationAnalysisContext context)
        {
            var operation = context.Operation;
            var values = operation switch
            {
                IBinaryOperation { OperatorKind: BinaryOperatorKind.Add, Type.SpecialType: SpecialType.System_String } add => new[] { add.LeftOperand, add.RightOperand },
                IInterpolationOperation hole => new[] { hole.Expression },
                IConversionOperation { Type.SpecialType: SpecialType.System_Object } conversion when WritesText(conversion) => new[] { conversion.Operand },
                _ => new IOperation[0],
            };

            foreach (var value in values.Select(Unwrap))
            {
                if (value.Type != null && IsHandle(value.Type) && !IsLocal(operation, context.ContainingSymbol) && !Silenced(operation, context.ContainingSymbol))
                {
                    context.ReportDiagnostic(Diagnostic.Create(Rules.LocalOutside, value.Syntax.GetLocation(), value.Syntax.ToString(), HandleText));
                }
            }
        }

        // An object argument of a library method that makes text of it (Console.WriteLine, string.Format, StringBuilder.Append).
        private static bool WritesText(IConversionOperation conversion)
        {
            var up = conversion.Parent;
            while (up is IArrayInitializerOperation or IArrayCreationOperation)
            {
                up = up.Parent;
            }

            return up is IArgumentOperation { Parent: IInvocationOperation invocation }
                && invocation.TargetMethod.Locations.All(l => !l.IsInSource)
                && invocation.TargetMethod.Name is "WriteLine" or "Write" or "Format" or "Concat" or "Append" or "AppendFormat" or "Join";
        }

        private bool IsHandle(ITypeSymbol type)
        {
            for (var at = type; at != null; at = at.BaseType)
            {
                if (handle != null && SymbolEqualityComparer.Default.Equals(at, handle))
                {
                    return true;
                }
            }

            return false;
        }

        // Whether a type is the one of a full metadata name, such as System.DateTime.
        private static bool IsNamed(INamedTypeSymbol? type, string fullName) =>
            type != null && type.ToDisplayString(SymbolDisplayFormat.FullyQualifiedFormat) == "global::" + fullName;

        /// <summary>
        /// Whether a read of a unit's name (not a hero's proper name, which has no declared value) is the declared one, the same on every machine: in a declared class's
        /// constructor on the object itself, or on a default object (<c>….Default</c> or <c>Unit.Default&lt;T&gt;()</c>).
        /// A copy of a default object in a variable is not known to be one.
        /// </summary>
        private bool DeclaredName(IOperation operation, ISymbol member, bool write)
        {
            if (write || member is not IPropertySymbol { Name: "Name" } property || !IsRuntime(property)
                || operation is not IPropertyReferenceOperation { Instance: { } instance })
            {
                return false;
            }

            if (instance is IInstanceReferenceOperation)
            {
                return InDeclaredConstructor(operation);
            }

            return instance switch
            {
                IPropertyReferenceOperation { Property: { Name: "Default", IsStatic: false } defaults } => IsRuntimeObject(defaults.ContainingType),
                IInvocationOperation { TargetMethod: { Name: "Default", IsGenericMethod: true } method } => IsRuntime(method),
                _ => false,
            };
        }

        // nameof(Name) names the member without reading it.
        private static bool InNameOf(IOperation operation)
        {
            for (var node = operation.Syntax.Parent; node != null && node is not StatementSyntax and not MemberDeclarationSyntax; node = node.Parent)
            {
                if (node is InvocationExpressionSyntax { Expression: IdentifierNameSyntax { Identifier.ValueText: "nameof" } } invocation
                    && operation.SemanticModel!.GetSymbolInfo(invocation).Symbol == null)
                {
                    return true;
                }
            }

            return false;
        }

        // A unit or item class of Moonwell or of Moonwell.Objects, whose Default is the one of the runtime's Unit or Item.
        private bool IsRuntimeObject(INamedTypeSymbol? type)
        {
            for (; type != null; type = type.BaseType)
            {
                if (IsRuntime(type))
                {
                    return true;
                }
            }

            return false;
        }

        private bool IsRuntime(ISymbol member) =>
            runtime != null && SymbolEqualityComparer.Default.Equals(member.ContainingAssembly, runtime);

        /// <summary>Whether an operation is in the body of an instance constructor of a declared class, and not in a lambda or local function there, which may run later.</summary>
        private static bool InDeclaredConstructor(IOperation operation)
        {
            for (var node = operation.Syntax; node != null; node = node.Parent)
            {
                switch (node)
                {
                    case AnonymousFunctionExpressionSyntax or LocalFunctionStatementSyntax:
                        return false;
                    case ConstructorDeclarationSyntax constructor:
                        return !constructor.Modifiers.Any(SyntaxKind.StaticKeyword)
                            && operation.SemanticModel!.GetDeclaredSymbol(constructor)?.ContainingType is { } type
                            && DeclaredAnalyzer.IsDeclared(type);
                    case MemberDeclarationSyntax:
                        return false;
                }
            }

            return false;
        }

        public void Loop(OperationAnalysisContext context)
        {
            if (context.Operation is IForEachLoopOperation loop && !Silenced(loop, context.ContainingSymbol)
                && HashName(loop.Collection) is { } hash)
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.HashLoop, loop.Collection.Syntax.GetLocation(), Article(hash)));
            }
        }

        public void Conversion(OperationAnalysisContext context)
        {
            var conversion = (IConversionOperation)context.Operation;
            var silenced = Silenced(conversion, context.ContainingSymbol);
            if (!conversion.IsImplicit && conversion.Type is { } target && Banned(target) is { } banned && (!silenced || banned == "decimal"))
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.NumberType, conversion.Syntax.GetLocation(), banned, Why(banned)));
            }

            if (silenced)
            {
                return;
            }

            if (HashName(conversion.Operand) is { } hash && IsCollection(conversion.Type)
                && !(conversion.Parent is IForEachLoopOperation loop && loop.Collection == conversion))
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.HashLoop, conversion.Operand.Syntax.GetLocation(), Article(hash)));
            }
        }

        /// <summary>MW0004 for the keys or values of a dictionary interface given as they are to an IEnumerable parameter (LINQ on them), where no conversion shows.</summary>
        public void HashArgument(OperationAnalysisContext context)
        {
            var argument = (IArgumentOperation)context.Operation;
            if (argument.Value is IPropertyReferenceOperation { Property.Name: "Keys" or "Values" } keys && HashName(keys) is { } hash
                && argument.Parameter != null && IsCollection(argument.Parameter.Type) && !Silenced(argument, context.ContainingSymbol))
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.HashLoop, keys.Syntax.GetLocation(), Article(hash)));
            }
        }

        public void Literal(OperationAnalysisContext context)
        {
            var literal = context.Operation;
            if (literal.Type is { } type && Banned(type) is { } banned && (banned == "decimal" || !Silenced(literal, context.ContainingSymbol)))
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.NumberType, literal.Syntax.GetLocation(), banned, Why(banned)));
            }
        }

        public void Declarator(OperationAnalysisContext context)
        {
            var declarator = (IVariableDeclaratorOperation)context.Operation;
            if (Banned(declarator.Symbol.Type) is { } banned && (banned == "decimal" || !Silenced(declarator, context.ContainingSymbol)))
            {
                var location = declarator.Symbol.Locations.FirstOrDefault() ?? declarator.Syntax.GetLocation();
                context.ReportDiagnostic(Diagnostic.Create(Rules.NumberType, location, banned, Why(banned)));
            }
        }

        public void Declaration(SymbolAnalysisContext context)
        {
            var symbol = context.Symbol;
            if (symbol.IsImplicitlyDeclared)
            {
                return;
            }

            var types = new List<ITypeSymbol>();
            switch (symbol)
            {
                case IFieldSymbol field:
                    types.Add(field.Type);
                    break;
                case IPropertySymbol property:
                    types.Add(property.Type);
                    types.AddRange(property.Parameters.Select(p => p.Type));
                    break;
                case IMethodSymbol method when method.AssociatedSymbol == null:
                    types.Add(method.ReturnType);
                    types.AddRange(method.Parameters.Select(p => p.Type));
                    break;
                case INamedTypeSymbol { EnumUnderlyingType: { } underlying }:
                    types.Add(underlying);
                    break;
            }

            // [SyncUnchecked] does not silence decimal: it is a number the game's Lua gets wrong on every machine.
            var banned = types.Select(Banned).FirstOrDefault(b => b != null);
            if (banned != null && (banned == "decimal" || !Has(symbol, syncUnchecked)) && symbol.Locations.FirstOrDefault() is { } location && location.IsInSource)
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.NumberType, location, banned, Why(banned)));
            }
        }

        /// <summary>Whether [SyncUnchecked] is on the member, local function or lambda around an operation.</summary>
        private bool Silenced(IOperation operation, ISymbol container)
        {
            if (syncUnchecked == null)
            {
                return false;
            }

            var model = operation.SemanticModel!;
            for (var node = operation.Syntax; node != null && node is not MemberDeclarationSyntax; node = node.Parent)
            {
                if (node is LocalFunctionStatementSyntax or AnonymousFunctionExpressionSyntax
                    && DeclaredBy(node, model) is { } function && Has(function, syncUnchecked))
                {
                    return true;
                }
            }

            return Has(container, syncUnchecked);
        }

        private static ISymbol? DeclaredBy(SyntaxNode node, SemanticModel model) =>
            node is LocalFunctionStatementSyntax ? model.GetDeclaredSymbol(node) : model.GetSymbolInfo(node).Symbol;

        private bool IsNative(ISymbol member) => natives != null && SymbolEqualityComparer.Default.Equals(member.ContainingType, natives);

        /// <summary>Whether a runtime member is marked local-safe: itself (a property's getter only), or its class.</summary>
        private bool IsLocalSafe(ISymbol member, bool write)
        {
            if (member is IMethodSymbol { AssociatedSymbol: IPropertySymbol property })
            {
                write = SymbolEqualityComparer.Default.Equals(property.SetMethod, member);
                member = property;
            }

            if (!write && Has(member, localSafe, false))
            {
                return true;
            }

            for (var type = member.ContainingType; type != null; type = type.ContainingType)
            {
                if (Has(type.OriginalDefinition, localSafe, false))
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>Whether a member is local to one machine: marked [MachineLocal] itself, or a property whose getter is, for a read only.</summary>
        private bool IsMachineLocal(ISymbol member, bool write) =>
            Has(member, machineLocal) || (!write && member is IPropertySymbol property && Has(property.GetMethod, machineLocal, false));

        /// <summary>Whether a symbol carries an attribute; an accessor also takes its property's.</summary>
        private static bool Has(ISymbol? symbol, INamedTypeSymbol? attribute, bool withProperty = true)
        {
            if (symbol == null || attribute == null)
            {
                return false;
            }

            if (symbol.GetAttributes().Any(a => SymbolEqualityComparer.Default.Equals(a.AttributeClass, attribute)))
            {
                return true;
            }

            return withProperty && symbol is IMethodSymbol { AssociatedSymbol: { } associated } && Has(associated, attribute, false);
        }

        private bool IsHash(ITypeSymbol type)
        {
            for (var current = type as INamedTypeSymbol; current != null; current = current.BaseType)
            {
                if (hashTypes.Any(h => h != null && SymbolEqualityComparer.Default.Equals(current.OriginalDefinition, h)))
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>
        /// The name of the type of a value that gives its items in no fixed order: a Dictionary or a HashSet (or a class
        /// derived from one, or a Dictionary's Keys or Values), an interface only those implement in the game's Lua
        /// (IDictionary, IReadOnlyDictionary, ISet, IReadOnlySet), or the Keys or Values of one. Null for any other.
        /// </summary>
        private string? HashName(IOperation value)
        {
            value = Unwrap(value);
            if (value.Type is not INamedTypeSymbol type)
            {
                return null;
            }

            if (IsHash(type) || hashInterfaces.Any(h => h != null && SymbolEqualityComparer.Default.Equals(type.OriginalDefinition, h)))
            {
                return type.Name;
            }

            return value is IPropertyReferenceOperation { Property.Name: "Keys" or "Values", Instance: { } instance } ? HashName(instance) : null;
        }

        // A Dictionary's name with its article: "A Dictionary", "An IReadOnlyDictionary".
        private static string Article(string name) => ("AEIOU".IndexOf(name[0]) >= 0 ? "An " : "A ") + name;

        // An interface a loop or a LINQ call goes through: IEnumerable, IEnumerable<T>, ICollection, ICollection<T>, IReadOnlyCollection<T>.
        private bool IsCollection(ITypeSymbol? type) =>
            type != null && collections.Any(c => c != null && SymbolEqualityComparer.Default.Equals(type.OriginalDefinition, c));

        private static IOperation Unwrap(IOperation operation)
        {
            while (operation is IConversionOperation { IsImplicit: true } conversion)
            {
                operation = conversion.Operand;
            }

            return operation;
        }

        /// <summary>The number type the game cannot carry that a type is or holds; null for none.</summary>
        private static string? Banned(ITypeSymbol type)
        {
            switch (type.SpecialType)
            {
                case SpecialType.System_Int64:
                    return "long";
                case SpecialType.System_UInt64:
                    return "ulong";
                case SpecialType.System_UInt32:
                    return "uint";
                case SpecialType.System_Double:
                    return "double";
                case SpecialType.System_Decimal:
                    return "decimal";
            }

            return type switch
            {
                IArrayTypeSymbol array => Banned(array.ElementType),
                INamedTypeSymbol named => named.TypeArguments.Select(Banned).FirstOrDefault(b => b != null),
                _ => null,
            };
        }

        private static string Why(string banned) => banned switch
        {
            "double" => "is a single-precision float in the game's Lua; use float",
            "uint" => "arithmetic becomes a float in the game's Lua; use int",
            "decimal" => "is an ordinary number in the game's Lua, so 0.1m + 0.2m is not 0.3m; use int or float",
            _ => "is a 32-bit integer in the game's Lua; use int",
        };

        private static string Name(ISymbol member) => member switch
        {
            IMethodSymbol { MethodKind: MethodKind.Constructor } constructor => constructor.ContainingType.Name,
            ILocalSymbol or IParameterSymbol => member.Name,
            IMethodSymbol { MethodKind: MethodKind.UserDefinedOperator or MethodKind.Conversion } method =>
                method.ContainingType.Name + "." + method.ToDisplayString(new SymbolDisplayFormat(memberOptions: SymbolDisplayMemberOptions.None)),
            _ when member.ContainingType is { Name: "Natives" } => member.Name,
            _ => member.ContainingType != null ? member.ContainingType.Name + "." + member.Name : member.Name,
        };
    }
}
