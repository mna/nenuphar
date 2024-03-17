### result: "?ad"
### fail: unsupported binary op: string + int

program:
	names:
		G
	constants:
		string "a"        # 0
		string "b"        # 1
		string "c"        # 2
		string "d"        # 3
		string "?"        # 4
		int 1             # 5
		string "result"   # 6

# defer
# 	G.result = G.result + 'd'
# end
# catch
#   G.result = G.result + 1
# end
# defer
#   G.result = G.result + 'a'
# end
#	G.result = '?'
# G.result = G.result + 1
function: top 9 0 # stack is at 4 when throw
	defers:
		9 40 1
		27 40 19
	catches:
		18 40 10
	code:
		JMP  9
		PREDECLARED 0 # G
		CONSTANT 6    # result
		DUP2
		INDEX
		CONSTANT  3  # 'd'
		PLUS
		SETINDEX     # G.result = G.result + 'd'
		DEFEREXIT

		# 9
		JMP  18
		PREDECLARED 0 # G
		CONSTANT 6    # result
		DUP2
		INDEX
		CONSTANT  5  # 1
		PLUS	       # throws
		SETINDEX     # G.result = G.result + 1
		CATCHJMP 0

		# 18
		JMP  27
		PREDECLARED 0 # G
		CONSTANT 6    # result
		DUP2
		INDEX
		CONSTANT  0  # 'a'
		PLUS
		SETINDEX     # G.result = G.result + 'a'
		DEFEREXIT

		# 27
		PREDECLARED 0 # G
		CONSTANT 6    # result
		CONSTANT 4    # '?'
		SETINDEX      # G.result = '?'
		PREDECLARED 0 # G
		CONSTANT 6    # result
		DUP2
		INDEX
		CONSTANT  5  # 1
		PLUS
		SETINDEX     # G.result = G.result + 1, throws
		NIL
		RUNDEFER
		RETURN

