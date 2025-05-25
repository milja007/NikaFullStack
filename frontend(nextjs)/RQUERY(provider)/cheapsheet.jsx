"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createUser } from "@/server/users"; // Assuming this path is correct

import { getUsers } from "@/client/userss"; // Assuming this path is correct

// interface dataType {
//   id: number;
//   name: string;
// }
const cheapsheet = () => {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ["todos"],
    queryFn: getUsers,
  });

  const mutation = useMutation({
    mutationFn: createUser,
    onSuccess: (data) => {
      console.log("Mutation successful:", data);
      queryClient.invalidateQueries({ queryKey: ["todos"] });
    },
    onError: (error) => {
      console.error("Mutation error:", error);
    },
  });

  return (
    <div className="text-center">
      <button
        className="bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-8 rounded-lg shadow-md hover:shadow-lg transition-all duration-300 text-lg mb-6 disabled:opacity-50"
        onClick={() => {
          const newUserName = `User ${Date.now().toString().slice(-5)}`;
          mutation.mutate({ id: Date.now(), name: newUserName });
        }}
        disabled={mutation.isPending}
      >
        {mutation.isPending ? "Creating User..." : "Create User (Demo)"}
      </button>
      {/* ... (rest of user list rendering) ... */}
      <div className="space-y-3 max-w-md mx-auto">
        {query.isLoading && (
          <div className="text-gray-600">Loading users...</div>
        )}
        {query.isError && (
          <div className="text-red-600">
            Error loading users. Please try again.
          </div>
        )}
        {query.data?.length === 0 && !query.isLoading && (
          <div className="text-gray-500">No users found.</div>
        )}
        {query.data?.map((user) => (
          <div
            key={user.id}
            className="p-4 bg-white rounded-lg shadow text-gray-700 border border-gray-200"
          >
            {user.name} (ID: {user.id})
          </div>
        ))}
      </div>
    </div>
  );
};

export default cheapsheet;
