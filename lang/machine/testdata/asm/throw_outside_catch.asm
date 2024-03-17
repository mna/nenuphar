### fail: unsupported binary op: int + string

program:
	names:
		G
	constants:
		int 1        # 0
		string "a"   # 1
		int 2        # 2
		string "result" # 3

# do
# 	catch
# 		G["result"] = 2
#			return
# 	end
#		x = fn()
# end
# G["return"] = x + 'a'
function: top 4 0
	locals:
		x
	catches:
		7 10 1
	code:
		JMP  7     # goto maketuple
		PREDECLARED 0 # G
		CONSTANT 3 # "result"
		CONSTANT 2 # 2
		SETINDEX   # G[result] = 2
		NIL
		RETURN      # no need to end with CATCHJMP as it would be unreachable

		MAKETUPLE 0 # no args
		MAKEFUNC 0  # fn
		CALL 0
		SETLOCAL 0  # x = fn()

		PREDECLARED 0 # G
		CONSTANT 3 # "result"
		LOCAL 0			# 1
		CONSTANT 1  # 'a'
		PLUS
		SETINDEX 
		NIL
		RETURN

# return 1
function: fn 1 0
	code:
		CONSTANT 0 # 1
		RETURN
