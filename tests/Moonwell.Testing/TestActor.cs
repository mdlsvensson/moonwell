namespace Moonwell.Testing
{
    /// <summary>An actor the cases add components to, so they can test the component list without the game.</summary>
    public sealed class TestActor : Actor
    {
        /// <summary>Adds a component to this actor and returns it.</summary>
        public T Add<T>(T component) where T : Component => AddComponent(component);
    }
}
